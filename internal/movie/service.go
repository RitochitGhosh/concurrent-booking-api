package movie

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/RitochitGhosh/seatbooking-api/internal/platform"
	"github.com/google/uuid"
)

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) Create(ctx context.Context, m Movie) (Movie, error) {
	m.Title, m.Synopsis, m.Genre = strings.TrimSpace(m.Title), strings.TrimSpace(m.Synopsis), strings.TrimSpace(m.Genre)
	if m.Title == "" || len(m.Title) > 120 || len(m.Synopsis) > 2000 || m.Genre == "" || len(m.Genre) > 50 || m.DurationMinutes < 1 || m.DurationMinutes > 600 || !m.StartsAt.After(time.Now()) || m.Rows < 1 || m.Rows > 12 || m.SeatsPerRow < 1 || m.SeatsPerRow > 16 {
		return Movie{}, fmt.Errorf("%w: provide a title, genre, future screening, duration 1–600, rows 1–12 and seats per row 1–16", platform.ErrInvalid)
	}
	m.PosterURL = strings.TrimSpace(m.PosterURL)
	if m.PosterURL != "" {
		u, err := url.ParseRequestURI(m.PosterURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || len(m.PosterURL) > 2048 {
			return Movie{}, fmt.Errorf("%w: poster_url must be an absolute HTTPS image URL (up to 2048 bytes)", platform.ErrInvalid)
		}
	}
	m.ID, m.CreatedAt = uuid.NewString(), time.Now().UTC()
	return m, s.store.Create(ctx, m)
}
func (s *Service) Get(ctx context.Context, id string) (Movie, error) { return s.store.Get(ctx, id) }
func (s *Service) List(ctx context.Context) ([]Movie, error) {
	movies, err := s.store.List(ctx)
	sort.Slice(movies, func(i, j int) bool { return movies[i].StartsAt.Before(movies[j].StartsAt) })
	return movies, err
}
