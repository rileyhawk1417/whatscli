#!/usr/bin/env python3
"""Drive whatscli in a pseudo-terminal and print the final screen.

Used for the manual QA checklist in 09-testing-and-qa.md: it lets you verify
startup, scrolling, resize and shutdown without a real terminal, and without a
WhatsApp session (the backend just prints "connecting..").

    ./pty_qa.py --keys '/help\r' --keys 'hello' [--bin ./whatscli]
    ./pty_qa.py --width 80 --height 24 --keys '...' --snapshot out.raw
"""
import argparse
import os
import pty
import re
import select
import struct
import fcntl
import termios
import time

ROWS, COLS = 30, 110


class Screen:
    """Minimal VT100 screen: cursor addressing, erase, and scroll regions."""

    def __init__(self, rows, cols):
        self.rows, self.cols = rows, cols
        self.grid = [[" "] * cols for _ in range(rows)]
        self.r = self.c = 0
        self.pending_wrap = False
        self.top, self.bottom = 0, rows - 1

    def put(self, ch):
        # Terminals defer the wrap until the next printable character, so a
        # full-width line followed by CR LF must not advance twice.
        if self.pending_wrap:
            self.pending_wrap = False
            self.c = 0
            self.linefeed()
        if self.r < 0:
            self.r = 0
        if self.r > self.bottom:
            self.scroll_up(1)
            self.r = self.bottom
        if 0 <= self.c < self.cols:
            self.grid[self.r][self.c] = ch
        self.c += 1
        if self.c >= self.cols:
            self.c = self.cols - 1
            self.pending_wrap = True

    def linefeed(self):
        if self.r == self.bottom:
            self.scroll_up(1)
        else:
            self.r = min(self.rows - 1, self.r + 1)

    def scroll_up(self, n):
        for _ in range(n):
            del self.grid[self.top]
            self.grid.insert(self.bottom, [" "] * self.cols)

    def scroll_down(self, n):
        for _ in range(n):
            del self.grid[self.bottom]
            self.grid.insert(self.top, [" "] * self.cols)

    def erase_in_line(self, mode):
        if mode == 1:
            for x in range(0, self.c + 1):
                self.grid[self.r][x] = " "
        elif mode == 2:
            self.grid[self.r] = [" "] * self.cols
        else:
            for x in range(self.c, self.cols):
                self.grid[self.r][x] = " "

    def erase_in_display(self, mode):
        if mode == 2:
            self.grid = [[" "] * self.cols for _ in range(self.rows)]
        elif mode == 1:
            for y in range(0, self.r + 1):
                self.grid[y] = [" "] * self.cols
        else:
            self.erase_in_line(0)
            for y in range(self.r + 1, self.rows):
                self.grid[y] = [" "] * self.cols

    def feed(self, data):
        i = 0
        n = len(data)
        while i < n:
            ch = data[i]
            if ch == "\x1b":
                m = re.compile(r"\x1b\[([0-9;?]*)([@-~])").match(data, i)
                if m:
                    self.csi(m.group(1), m.group(2))
                    i = m.end()
                    continue
                m = re.compile(r"\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)?").match(data, i)
                if m:
                    i = m.end()
                    continue
                i += 2  # charset selection etc.
                continue
            if ch == "\r":
                self.c = 0
                self.pending_wrap = False
            elif ch == "\n":
                self.pending_wrap = False
                self.linefeed()
            elif ch == "\b":
                self.c = max(0, self.c - 1)
            elif ch == "\t":
                self.c = min(self.cols - 1, (self.c // 8 + 1) * 8)
            elif ch >= " ":
                self.put(ch)
            i += 1

    def csi(self, params, cmd):
        nums = [int(p) if p.isdigit() else 0 for p in params.split(";")] if params else []
        first = nums[0] if nums else 0
        self.pending_wrap = False
        if cmd == "H" or cmd == "f":
            self.r = (nums[0] if len(nums) > 0 and nums[0] else 1) - 1
            self.c = (nums[1] if len(nums) > 1 and nums[1] else 1) - 1
        elif cmd == "G":
            self.c = (first or 1) - 1
        elif cmd == "d":
            self.r = (first or 1) - 1
        elif cmd == "A":
            self.r = max(self.top, self.r - (first or 1))
        elif cmd == "B":
            self.r = min(self.bottom, self.r + (first or 1))
        elif cmd == "C":
            self.c = min(self.cols - 1, self.c + (first or 1))
        elif cmd == "D":
            self.c = max(0, self.c - (first or 1))
        elif cmd == "E":
            self.r = min(self.bottom, self.r + (first or 1))
            self.c = 0
        elif cmd == "F":
            self.r = max(self.top, self.r - (first or 1))
            self.c = 0
        elif cmd == "J":
            self.erase_in_display(first)
        elif cmd == "K":
            self.erase_in_line(first)
        elif cmd == "L":
            for _ in range(first or 1):
                del self.grid[self.bottom]
                self.grid.insert(self.r, [" "] * self.cols)
        elif cmd == "M":
            for _ in range(first or 1):
                del self.grid[self.r]
                self.grid.insert(self.bottom, [" "] * self.cols)
        elif cmd == "P":
            row = self.grid[self.r]
            for _ in range(first or 1):
                del row[self.c]
                row.append(" ")
        elif cmd == "X":
            for x in range(self.c, min(self.cols, self.c + (first or 1))):
                self.grid[self.r][x] = " "
        elif cmd == "S":
            self.scroll_up(first or 1)
        elif cmd == "T":
            self.scroll_down(first or 1)
        elif cmd == "r":
            self.top = (nums[0] if len(nums) > 0 and nums[0] else 1) - 1
            self.bottom = (nums[1] if len(nums) > 1 and nums[1] else self.rows) - 1
            self.r = self.c = 0

    def dump(self):
        return "\n".join("".join(row).rstrip() for row in self.grid)


def run(binary, keys, width, height, settle, start_wait):
    pid, fd = pty.fork()
    if pid == 0:
        os.environ.setdefault("TERM", "xterm-256color")
        os.execv(binary, [binary])

    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", height, width, 0, 0))
    os.kill(pid, 28)  # SIGWINCH

    out = bytearray()

    def pump(seconds):
        end = time.time() + seconds
        while time.time() < end:
            ready, _, _ = select.select([fd], [], [], 0.05)
            if not ready:
                continue
            try:
                chunk = os.read(fd, 65536)
            except OSError:
                return False
            if not chunk:
                return False
            out.extend(chunk)
            # Answer the terminal capability queries lipgloss sends on startup.
            if b"\x1b]11;?" in chunk:
                os.write(fd, b"\x1b]11;rgb:0000/0000/0000\x1b\\")
            if b"\x1b[6n" in chunk:
                os.write(fd, b"\x1b[1;1R")
        return True

    pump(start_wait)
    for key in keys:
        os.write(fd, key.encode().decode("unicode_escape").encode())
        pump(settle)
    os.write(fd, b"\x11")  # Ctrl+q
    pump(settle)
    try:
        os.kill(pid, 9)
    except ProcessLookupError:
        pass
    return bytes(out)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--bin", default="./whatscli")
    ap.add_argument("--keys", action="append", default=[])
    ap.add_argument("--width", type=int, default=COLS)
    ap.add_argument("--height", type=int, default=ROWS)
    ap.add_argument("--settle", type=float, default=0.6)
    ap.add_argument("--start-wait", type=float, default=2.5)
    ap.add_argument("--snapshot", help="write the raw byte stream here")
    args = ap.parse_args()

    raw = run(os.path.abspath(args.bin), args.keys, args.width, args.height,
              args.settle, args.start_wait)
    if args.snapshot:
        with open(args.snapshot, "wb") as fh:
            fh.write(raw)
    screen = Screen(args.height, args.width)
    screen.feed(raw.decode("utf-8", "replace"))
    print(screen.dump())


if __name__ == "__main__":
    main()
