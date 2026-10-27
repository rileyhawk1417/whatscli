package messages

import (
	"errors"
	"strings"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
)

// newQRTestSessionManager builds the minimum SessionManager the QR event loop
// needs: a handler to record output and a buffered status channel so the loop
// never blocks on an unconsumed notification.
func newQRTestSessionManager(t *testing.T) (*SessionManager, *MockUiHandler) {
	t.Helper()
	sm, mock := newTestSessionManager(t)
	sm.StatusChannel = make(chan StatusMsg, 10)
	return sm, mock
}

// awaitQR consumes the loop's error with a deadline so a regression back to the
// infinite spin fails the test instead of hanging the suite.
func awaitQR(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("handleQRChannel did not return; the loop is spinning on a closed channel")
		return nil
	}
}

func TestQRChannelTerminalEventReturnsError(t *testing.T) {
	tests := []struct {
		name    string
		item    whatsmeow.QRChannelItem
		wantErr string
	}{
		{
			name:    "client outdated",
			item:    whatsmeow.QRChannelClientOutdated,
			wantErr: "client-outdated",
		},
		{
			name:    "scanned without multidevice",
			item:    whatsmeow.QRChannelScannedWithoutMultidevice,
			wantErr: "scanned-without-multidevice",
		},
		{
			name:    "pairing error",
			item:    whatsmeow.QRChannelItem{Event: "error", Error: errors.New("pairing refused")},
			wantErr: "pairing refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm, _ := newQRTestSessionManager(t)
			ch := make(chan whatsmeow.QRChannelItem, 1)

			done := make(chan error, 1)
			go func() { done <- sm.handleQRChannel(ch) }()

			ch <- tt.item
			close(ch)

			err := awaitQR(t, done)
			if err == nil {
				t.Fatalf("expected an error for %s", tt.name)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not mention %q", err, tt.wantErr)
			}

			// The failure must reach the status bar, not just the log.
			select {
			case st := <-sm.StatusChannel:
				if st.connected {
					t.Fatalf("status reported connected after a pairing failure")
				}
			default:
				t.Fatalf("no status update was queued for the pairing failure")
			}
		})
	}
}

func TestQRChannelClosedWithoutEventReturnsError(t *testing.T) {
	sm, _ := newQRTestSessionManager(t)
	ch := make(chan whatsmeow.QRChannelItem)
	close(ch)

	done := make(chan error, 1)
	go func() { done <- sm.handleQRChannel(ch) }()

	err := awaitQR(t, done)
	if err == nil {
		t.Fatal("expected an error when the QR channel closes")
	}
	if !strings.Contains(err.Error(), "closed") {
		t.Fatalf("error %q should explain the connection closed", err)
	}
}

func TestQRChannelSuccessReturnsNil(t *testing.T) {
	sm, mock := newQRTestSessionManager(t)
	ch := make(chan whatsmeow.QRChannelItem, 1)

	done := make(chan error, 1)
	go func() { done <- sm.handleQRChannel(ch) }()

	ch <- whatsmeow.QRChannelSuccess

	if err := awaitQR(t, done); err != nil {
		t.Fatalf("success event returned an error: %v", err)
	}
	if !mock.HasText("Successfully logged in!") {
		t.Fatal("success was not reported to the UI")
	}
}

func TestQRChannelTimeoutKeepsWaiting(t *testing.T) {
	sm, mock := newQRTestSessionManager(t)
	ch := make(chan whatsmeow.QRChannelItem, 2)

	done := make(chan error, 1)
	go func() { done <- sm.handleQRChannel(ch) }()

	ch <- whatsmeow.QRChannelTimeout
	ch <- whatsmeow.QRChannelSuccess

	if err := awaitQR(t, done); err != nil {
		t.Fatalf("timeout should not end the loop: %v", err)
	}
	if !mock.HasText("QR code timeout. Waiting for new code...") {
		t.Fatal("timeout was not reported to the UI")
	}
}
