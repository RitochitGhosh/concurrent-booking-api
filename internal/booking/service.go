package booking

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/RitochitGhosh/seatbooking-api/internal/movie"
	"github.com/RitochitGhosh/seatbooking-api/internal/platform"
)

type MovieReader interface {
	Get(context.Context, string) (movie.Movie, error)
}
type Service struct {
	store  BookingStore
	movies MovieReader
}

func NewService(store BookingStore, movies MovieReader) *Service {
	return &Service{store: store, movies: movies}
}

func (s *Service) Hold(ctx context.Context, b Booking) (Booking, error) {
	m, err := s.movies.Get(ctx, b.MovieID)
	if err != nil {
		return Booking{}, err
	}
	if !m.StartsAt.After(time.Now()) {
		return Booking{}, fmt.Errorf("%w: screening has already started", platform.ErrInvalid)
	}
	if !validSeat(b.SeatID, m) {
		return Booking{}, fmt.Errorf("%w: seat does not exist", platform.ErrInvalid)
	}
	return s.store.Hold(ctx, b)
}
func (s *Service) Confirm(ctx context.Context, movieID, seatID, id, userID string) (Booking, error) {
	m, err := s.movies.Get(ctx, movieID)
	if err != nil {
		return Booking{}, err
	}
	if !m.StartsAt.After(time.Now()) {
		return Booking{}, fmt.Errorf("%w: screening has already started", platform.ErrInvalid)
	}
	if !validSeat(seatID, m) {
		return Booking{}, platform.ErrInvalid
	}
	return s.store.Confirm(ctx, movieID, seatID, id, userID)
}
func (s *Service) Release(ctx context.Context, movieID, seatID, id, userID string) error {
	m, err := s.movies.Get(ctx, movieID)
	if err != nil {
		return err
	}
	if !validSeat(seatID, m) {
		return platform.ErrInvalid
	}
	return s.store.Release(ctx, movieID, seatID, id, userID)
}

func (s *Service) List(ctx context.Context, movieID string) ([]Booking, error) {
	if _, err := s.movies.Get(ctx, movieID); err != nil {
		return nil, err
	}
	return s.store.List(ctx, movieID)
}
func validSeat(seat string, m movie.Movie) bool {
	if len(seat) < 2 || seat[0] < 'A' || int(seat[0]-'A') >= m.Rows {
		return false
	}
	n, err := strconv.Atoi(seat[1:])
	return err == nil && n >= 1 && n <= m.SeatsPerRow && seat == fmt.Sprintf("%c%d", seat[0], n)
}
