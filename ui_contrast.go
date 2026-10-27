package main

// ui_contrast.go — keeps configured colors legible against the configured
// background.
//
// The palette predates the Bubbletea UI: it was written for tcell, where a name
// like "blue" means the *dark* ANSI tone. On a black terminal that is 2.4:1 —
// well under the 4.5:1 that WCAG AA asks for body text, so sender names and
// forwarded messages come out barely readable. Rather than break those configs
// by silently shipping different colors, the hue is kept and only its lightness
// is raised until the text clears the threshold. `auto_contrast = false` in the
// [colors] section turns this off and uses the configured values verbatim.

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// minContrast is the WCAG AA ratio for normal-sized text.
const minContrast = 4.5

// dimMix is how far the timestamp/secondary tone is pulled from the text color
// towards the background before the contrast floor is applied.
const dimMix = 0.45

type rgb struct{ r, g, b uint8 }

func (c rgb) hex() string { return fmt.Sprintf("#%02x%02x%02x", c.r, c.g, c.b) }

// parseColor decodes a resolved config value into RGB. Names have already been
// mapped to hex by resolveColor, so only hex strings and ANSI indices reach
// here.
func parseColor(value string) (rgb, bool) {
	v := strings.TrimSpace(strings.ToLower(value))
	if strings.HasPrefix(v, "#") {
		h := v[1:]
		if len(h) == 3 {
			h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
		}
		if len(h) != 6 {
			return rgb{}, false
		}
		var parts [3]uint8
		for i := 0; i < 3; i++ {
			n, err := strconv.ParseUint(h[i*2:i*2+2], 16, 8)
			if err != nil {
				return rgb{}, false
			}
			parts[i] = uint8(n)
		}
		return rgb{parts[0], parts[1], parts[2]}, true
	}
	if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 255 {
		return xtermRGB(n), true
	}
	return rgb{}, false
}

// xtermRGB expands an ANSI index into the color the terminal will actually
// paint for it (the standard 256-color cube).
func xtermRGB(n int) rgb {
	if n < 16 {
		basic := [16]rgb{
			{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0},
			{0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192},
			{128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
			{0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
		}
		return basic[n]
	}
	if n < 232 {
		n -= 16
		steps := [6]uint8{0, 95, 135, 175, 215, 255}
		return rgb{steps[n/36], steps[(n/6)%6], steps[n%6]}
	}
	g := uint8(8 + 10*(n-232))
	return rgb{g, g, g}
}

func srgbToLinear(v uint8) float64 {
	c := float64(v) / 255
	if c <= 0.03928 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// relativeLuminance implements the WCAG definition (0 = black, 1 = white).
func (c rgb) relativeLuminance() float64 {
	return 0.2126*srgbToLinear(c.r) + 0.7152*srgbToLinear(c.g) + 0.0722*srgbToLinear(c.b)
}

// contrastRatio returns the WCAG contrast ratio between two colors (1:1 to
// 21:1).
func contrastRatio(a, b rgb) float64 {
	la, lb := a.relativeLuminance(), b.relativeLuminance()
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// mixColor blends a towards b; t=0 returns a, t=1 returns b.
func mixColor(a, b rgb, t float64) rgb {
	blend := func(x, y uint8) uint8 {
		return uint8(math.Round(float64(x)*(1-t) + float64(y)*t))
	}
	return rgb{blend(a.r, b.r), blend(a.g, b.g), blend(a.b, b.b)}
}

// ensureContrast raises fg away from bg until it clears min, keeping the hue
// and only changing lightness. It reports whether the color had to change; the
// returned color is the closest one reachable by moving towards white (on a
// dark background) or black (on a light one).
func ensureContrast(fg, bg rgb, min float64) (rgb, bool) {
	if contrastRatio(fg, bg) >= min {
		return fg, false
	}
	target := rgb{255, 255, 255}
	if bg.relativeLuminance() > 0.18 {
		target = rgb{0, 0, 0}
	}
	const steps = 32
	best := fg
	for i := 1; i <= steps; i++ {
		best = mixColor(fg, target, float64(i)/steps)
		if contrastRatio(best, bg) >= min {
			return best, true
		}
	}
	return best, true
}

// dimColor derives the secondary tone (timestamps, hints) from the palette: a
// blend of the text color towards the background, never below the contrast
// floor, so "dim" stays readable instead of disappearing.
func dimColor(text, background rgb) rgb {
	dim, _ := ensureContrast(mixColor(text, background, dimMix), background, minContrast)
	return dim
}

// contrastNote describes one adjusted palette entry for the startup notice.
func contrastNote(name string, from, to, background rgb) string {
	return fmt.Sprintf("%s %s→%s (%.1f:1)", name, from.hex(), to.hex(), contrastRatio(to, background))
}
