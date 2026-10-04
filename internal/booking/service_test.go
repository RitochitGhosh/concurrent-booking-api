package booking

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RitochitGhosh/seatbooking-api/internal/movie"
	"github.com/RitochitGhosh/seatbooking-api/internal/platform"
)

// The earlier in-process store remains a small concurrency example.
func TestConcurrentBookingExactlyOneWins(t *testing.T) {
	store := NewConcurrentStore()
	var successes atomic.Int64
	var wg sync.WaitGroup
	for range 1000 {
		wg.Go(func() {
			if err := store.Book(Booking{MovieID: "screen-1", SeatID: "A1"}); err == nil {
				successes.Add(1)
			} else if !errors.Is(err, ErrSeatAlreadyBooked) {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("successes = %d, want 1", successes.Load())
	}
}

type movieStub struct{ m movie.Movie }

func (s movieStub) Get(context.Context, string) (movie.Movie, error) { return s.m, nil }

type bookingStub struct{ calls int }

func (s *bookingStub) Hold(_ context.Context, b Booking) (Booking, error) { s.calls++; return b, nil }
func (*bookingStub) Confirm(context.Context, string, string, string, string) (Booking, error) {
	return Booking{}, nil
}
func (*bookingStub) Release(context.Context, string, string, string, string) error { return nil }
func (*bookingStub) List(context.Context, string) ([]Booking, error)               { return nil, nil }
func TestHoldValidatesScreeningAndSeat(t *testing.T) {
	m := movie.Movie{Rows: 6, SeatsPerRow: 10, StartsAt: time.Now().Add(time.Hour)}
	store := &bookingStub{}
	svc := NewService(store, movieStub{m})
	for _, seat := range []string{"", "A0", "A01", "A11", "G1", "a1", "A-1"} {
		if _, err := svc.Hold(context.Background(), Booking{SeatID: seat}); !errors.Is(err, platform.ErrInvalid) {
			t.Errorf("seat %q accepted: %v", seat, err)
		}
	}
	if store.calls != 0 {
		t.Fatal("invalid seats reached the store")
	}
	if _, err := svc.Hold(context.Background(), Booking{SeatID: "F10"}); err != nil {
		t.Fatal(err)
	}
	m.StartsAt = time.Now().Add(-time.Hour)
	svc = NewService(store, movieStub{m})
	if _, err := svc.Hold(context.Background(), Booking{SeatID: "A1"}); !errors.Is(err, platform.ErrInvalid) {
		t.Fatal("past screening accepted")
	}
}
