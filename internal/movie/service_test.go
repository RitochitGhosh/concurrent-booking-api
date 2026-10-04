package movie

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RitochitGhosh/seatbooking-api/internal/platform"
)

type storeStub struct{ saved Movie }

func (s *storeStub) Create(_ context.Context, m Movie) error  { s.saved = m; return nil }
func (*storeStub) Get(context.Context, string) (Movie, error) { return Movie{}, nil }
func (*storeStub) List(context.Context) ([]Movie, error)      { return nil, nil }

func TestPosterURLValidation(t *testing.T) {
	store := &storeStub{}
	service := NewService(store)
	m := Movie{Title: "Movie", Genre: "Drama", DurationMinutes: 120, StartsAt: time.Now().Add(time.Hour), Rows: 6, SeatsPerRow: 10}
	for _, value := range []string{"javascript:alert(1)", "data:image/png;base64,x", "/poster.jpg", "http://example.com/poster.jpg", "https://user:password@example.com/poster.jpg", "https://"} {
		m.PosterURL = value
		if _, err := service.Create(context.Background(), m); !errors.Is(err, platform.ErrInvalid) {
			t.Errorf("accepted %q: %v", value, err)
		}
	}
	for _, value := range []string{"", "https://example.com/poster.jpg?size=large"} {
		m.PosterURL = value
		result, err := service.Create(context.Background(), m)
		if err != nil || result.PosterURL != value || store.saved.PosterURL != value {
			t.Fatalf("poster %q not saved: %+v, %v", value, result, err)
		}
	}
}
