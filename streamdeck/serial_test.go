package streamdeck

import (
	"io"
	"os"
	"testing"
	"testing/synctest"
	"time"
)

func TestSerialNumberFollowsTheOpenedDeck(t *testing.T) {
	const (
		firstSerial       = "AL12A2B3C4D5"
		replacementSerial = "AL12E6F7G8H9"
	)
	for _, tc := range []struct {
		product uint16
		id      byte
		size    int
		offset  int
	}{
		{0x0060, 0x03, 17, 5},
		{0x0063, 0x03, 17, 5},
		{0x006c, 0x06, 32, 2},
		{0x006d, 0x06, 32, 2},
		{0x0080, 0x06, 32, 2},
		{0x0084, 0x06, 32, 2},
		{0x0090, 0x03, 32, 5},
	} {
		spec, ok := lookupModel(tc.product)
		if !ok {
			t.Fatalf("missing model for product 0x%04x", tc.product)
		}
		t.Run(spec.model.Name, func(t *testing.T) {
			for _, attachment := range []struct{ path, serial string }{
				{"/dev/hidraw0", firstSerial},
				{"/dev/hidraw1", firstSerial},
				{"/dev/hidraw0", replacementSerial},
			} {
				wire := &testTransport{
					closed: make(chan struct{}),
					feature: func(report []byte) (int, error) {
						if len(report) != tc.size || report[0] != tc.id {
							t.Fatalf("feature request = %x, want ID %#02x and %d bytes", report, tc.id, tc.size)
						}
						if tc.id == 0x06 {
							report[1] = byte(len(attachment.serial))
						} else {
							copy(report[1:], []byte{1, 2, 3, 4}) // Reserved bytes precede the legacy serial.
						}
						copy(report[tc.offset:], attachment.serial)
						return len(report), nil
					},
				}
				deck := newDevice(attachment.path, wire, spec)
				serial := deck.SerialNumber()
				if closeErr := deck.Close(); closeErr != nil {
					t.Fatal(closeErr)
				}
				if serial != attachment.serial {
					t.Fatalf("serial for attachment %s = %q, want %q", attachment.path, serial, attachment.serial)
				}
			}
		})
	}
}

func TestSerialNumberRecoversTransientFailuresAndReturnsEmptyWhenUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name        string
		response    []byte
		readErr     error
		recover     bool
		want        string
		wantQueries int
		wantDelay   time.Duration
	}{
		{name: "empty report", response: []byte{0x06, 0}, wantQueries: 1},
		{name: "unsupported report", readErr: ErrUnsupported, wantQueries: 1},
		{name: "permission denied", readErr: os.ErrPermission, wantQueries: 1},
		{name: "truncated serial", response: []byte{0x06, 12, 'A'}, wantQueries: 1},
		{name: "persistent failure", readErr: io.ErrNoProgress, wantQueries: 3, wantDelay: 30 * time.Millisecond},
		{
			name: "two failures then a valid report", readErr: io.ErrNoProgress, recover: true,
			want: "AL12A2B3C4D5E6", wantQueries: 3, wantDelay: 30 * time.Millisecond,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				deck, wire := testDeck(t, nil)
				queries := 0
				wire.feature = func(report []byte) (int, error) {
					queries++
					if report[0] != 0x06 || report[1] != 0 {
						t.Fatalf("serial query %d reused a partial response", queries)
					}
					if tc.recover && queries == 3 {
						report[1] = 14
						return 2 + copy(report[2:], "AL12A2B3C4D5E6"), nil
					}
					if tc.readErr != nil {
						report[0], report[1] = 0, 0xff // A failed transfer can still change its buffer.
						return 0, tc.readErr
					}
					return copy(report, tc.response), nil
				}
				start := time.Now()
				if serial := deck.SerialNumber(); serial != tc.want {
					t.Fatalf("SerialNumber() = %q, want %q", serial, tc.want)
				}
				if queries != tc.wantQueries || time.Since(start) != tc.wantDelay {
					t.Fatalf("serial query took %d attempts in %v, want %d in %v", queries, time.Since(start), tc.wantQueries, tc.wantDelay)
				}
			})
		})
	}
}

func TestCloseInterruptsSerialRetry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		deck, wire := testDeck(t, nil)
		queries := 0
		wire.feature = func([]byte) (int, error) {
			queries++
			return 0, io.ErrNoProgress
		}
		result := make(chan string, 1)
		go func() { result <- deck.SerialNumber() }()
		synctest.Wait()
		if err := deck.Close(); err != nil {
			t.Fatal(err)
		}
		if serial := <-result; serial != "" || queries != 1 {
			t.Fatalf("closing during a serial query yielded %q after %d attempts, want empty after one", serial, queries)
		}
		if serial := deck.SerialNumber(); serial != "" || queries != 1 {
			t.Fatalf("querying a closed deck yielded %q after %d attempts, want empty with no additional attempt", serial, queries)
		}
	})
}
