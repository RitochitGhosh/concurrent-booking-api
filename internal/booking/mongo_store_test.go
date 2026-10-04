package booking

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/RitochitGhosh/seatbooking-api/internal/platform"
	"github.com/RitochitGhosh/seatbooking-api/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMongoBookingLifecycle(t *testing.T) {
	db := testutil.Mongo(t)
	store := NewMongoStore(db)
	ctx := context.Background()
	if err := store.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	winners := make(chan Booking, 100)
	for range 100 {
		wg.Go(func() {
			b, err := store.Hold(ctx, Booking{MovieID: "screen-1", SeatID: "A1", UserID: "owner"})
			if err == nil {
				winners <- b
			} else if !errors.Is(err, ErrSeatAlreadyBooked) {
				t.Errorf("hold: %v", err)
			}
		})
	}
	wg.Wait()
	close(winners)
	if len(winners) != 1 {
		t.Fatalf("winners = %d", len(winners))
	}
	b := <-winners
	if _, err := store.Confirm(ctx, b.MovieID, b.SeatID, b.ID, "attacker"); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("foreign confirmation: %v", err)
	}
	confirmed, err := store.Confirm(ctx, b.MovieID, b.SeatID, b.ID, b.UserID)
	if err != nil || confirmed.Status != "confirmed" || !confirmed.ExpiresAt.IsZero() {
		t.Fatalf("confirmation = %+v, %v", confirmed, err)
	}
	if retry, err := store.Confirm(ctx, b.MovieID, b.SeatID, b.ID, b.UserID); err != nil || retry.ID != b.ID {
		t.Fatalf("idempotent confirmation: %v", err)
	}
	if _, err := store.Hold(ctx, Booking{MovieID: b.MovieID, SeatID: b.SeatID}); !errors.Is(err, ErrSeatAlreadyBooked) {
		t.Fatalf("confirmed seat reclaimed: %v", err)
	}
	if _, err := store.Hold(ctx, Booking{MovieID: "screen-2", SeatID: b.SeatID}); err != nil {
		t.Fatalf("independent screening: %v", err)
	}

	old, err := store.Hold(ctx, Booking{MovieID: "screen-1", SeatID: "A2", UserID: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("bookings").UpdateOne(ctx, bson.M{"id": old.ID}, bson.M{"$set": bson.M{"expires_at": time.Now().Add(-time.Second)}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Confirm(ctx, old.MovieID, old.SeatID, old.ID, old.UserID); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("expired hold confirmed: %v", err)
	}
	live, err := store.List(ctx, old.MovieID)
	if err != nil || len(live) != 1 {
		t.Fatalf("expired hold listed: %+v, %v", live, err)
	}
	fresh, err := store.Hold(ctx, Booking{MovieID: old.MovieID, SeatID: old.SeatID, UserID: "new-owner"})
	if err != nil || fresh.ID == old.ID {
		t.Fatalf("reclaim expired hold: %+v, %v", fresh, err)
	}
	if _, err := store.Confirm(ctx, old.MovieID, old.SeatID, old.ID, old.UserID); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("stale hold confirmed: %v", err)
	}
}

func TestMongoReleaseHold(t *testing.T) {
	store := NewMongoStore(testutil.Mongo(t))
	ctx := context.Background()
	if err := store.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	b, err := store.Hold(ctx, Booking{MovieID: "screen", SeatID: "A1", UserID: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Release(ctx, b.MovieID, b.SeatID, b.ID, "other"); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("foreign release: %v", err)
	}
	if err := store.Release(ctx, b.MovieID, b.SeatID, b.ID, b.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Confirm(ctx, b.MovieID, b.SeatID, b.ID, b.UserID); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("released hold confirmed: %v", err)
	}
	fresh, err := store.Hold(ctx, Booking{MovieID: b.MovieID, SeatID: b.SeatID, UserID: "new-owner"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Release(ctx, b.MovieID, b.SeatID, b.ID, b.UserID); err != nil {
		t.Fatalf("retry release: %v", err)
	}
	if _, err := store.Confirm(ctx, fresh.MovieID, fresh.SeatID, fresh.ID, fresh.UserID); err != nil {
		t.Fatalf("stale release deleted replacement: %v", err)
	}
	if err := store.Release(ctx, fresh.MovieID, fresh.SeatID, fresh.ID, fresh.UserID); !errors.Is(err, ErrHoldNotActive) {
		t.Fatalf("confirmed reservation released: %v", err)
	}
	if _, err := store.Hold(ctx, Booking{MovieID: fresh.MovieID, SeatID: fresh.SeatID}); !errors.Is(err, ErrSeatAlreadyBooked) {
		t.Fatal("confirmed seat became available")
	}
}

func TestMongoReleaseVersusConfirm(t *testing.T) {
	store := NewMongoStore(testutil.Mongo(t))
	ctx := context.Background()
	if err := store.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	b, err := store.Hold(ctx, Booking{MovieID: "screen", SeatID: "A1", UserID: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() { <-start; results <- store.Release(ctx, b.MovieID, b.SeatID, b.ID, b.UserID) }()
	go func() { <-start; _, err := store.Confirm(ctx, b.MovieID, b.SeatID, b.ID, b.UserID); results <- err }()
	close(start)
	first, second := <-results, <-results
	if (first == nil) == (second == nil) {
		t.Fatalf("expected exactly one successful transition: %v, %v", first, second)
	}
	for _, err := range []error{first, second} {
		if err != nil && !errors.Is(err, platform.ErrNotFound) && !errors.Is(err, ErrHoldNotActive) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}
