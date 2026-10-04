package booking

import (
	"context"
	"errors"
	"time"
)

const defaultHoldTTL = 2 * time.Minute

var ErrSeatAlreadyBooked = errors.New("seat is already taken")
var ErrHoldNotActive = errors.New("confirmed bookings cannot be released")

// Booking is either a temporary hold or an explicitly confirmed reservation.
type Booking struct {
	ID        string    `json:"id" bson:"id"`
	MovieID   string    `json:"movie_id" bson:"movie_id"`
	SeatID    string    `json:"seat_id" bson:"seat_id"`
	UserID    string    `json:"user_id" bson:"user_id"`
	Status    string    `json:"status" bson:"status"`
	ExpiresAt time.Time `json:"expires_at" bson:"expires_at"`
}

type BookingStore interface {
	Hold(context.Context, Booking) (Booking, error)
	Confirm(context.Context, string, string, string, string) (Booking, error)
	Release(context.Context, string, string, string, string) error
	List(context.Context, string) ([]Booking, error)
}
