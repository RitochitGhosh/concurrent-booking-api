package httpapi

import (
	"net/http"
	"time"

	"github.com/RitochitGhosh/seatbooking-api/internal/booking"
	"github.com/RitochitGhosh/seatbooking-api/internal/movie"
)

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.rdb.Ping(r.Context()).Err(); err != nil {
		writeJSON(w, 503, map[string]string{"status": "unavailable"})
		return
	}
	if err := s.mongoHealth(r.Context()); err != nil {
		writeJSON(w, 503, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}
func (s *Server) listMovies(w http.ResponseWriter, r *http.Request) {
	movies, err := s.movies.List(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, movies)
}
func (s *Server) createMovie(w http.ResponseWriter, r *http.Request) {
	// A dedicated input type prevents clients from supplying IDs or timestamps.
	var input struct {
		Title           string    `json:"title"`
		PosterURL       string    `json:"poster_url"`
		Synopsis        string    `json:"synopsis"`
		Genre           string    `json:"genre"`
		DurationMinutes int       `json:"duration_minutes"`
		StartsAt        time.Time `json:"starts_at"`
		Rows            int       `json:"rows"`
		SeatsPerRow     int       `json:"seats_per_row"`
	}
	if err := decode(w, r, &input); err != nil {
		s.fail(w, r, err)
		return
	}
	m, err := s.movies.Create(r.Context(), movie.Movie{Title: input.Title, PosterURL: input.PosterURL, Synopsis: input.Synopsis, Genre: input.Genre, DurationMinutes: input.DurationMinutes, StartsAt: input.StartsAt, Rows: input.Rows, SeatsPerRow: input.SeatsPerRow})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/movies/"+m.ID+"/seats")
	writeJSON(w, 201, m)
}
func (s *Server) listSeats(w http.ResponseWriter, r *http.Request) {
	bookings, err := s.bookings.List(r.Context(), r.PathValue("movieID"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	// Other users' identities and booking IDs never leave the server.
	type seat struct {
		SeatID    string     `json:"seat_id"`
		Status    string     `json:"status"`
		Mine      bool       `json:"mine"`
		BookingID string     `json:"booking_id,omitempty"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}
	seats := make([]seat, 0, len(bookings))
	for _, b := range bookings {
		v := seat{SeatID: b.SeatID, Status: b.Status, Mine: b.UserID == currentUser(r).ID}
		if v.Mine {
			v.BookingID = b.ID
			if b.Status == "held" {
				v.ExpiresAt = &b.ExpiresAt
			}
		}
		seats = append(seats, v)
	}
	writeJSON(w, 200, seats)
}
func (s *Server) hold(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SeatID string `json:"seat_id"`
	}
	if err := decode(w, r, &input); err != nil {
		s.fail(w, r, err)
		return
	}
	b, err := s.bookings.Hold(r.Context(), booking.Booking{MovieID: r.PathValue("movieID"), SeatID: input.SeatID, UserID: currentUser(r).ID})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 201, b)
}
func (s *Server) confirm(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SeatID string `json:"seat_id"`
	}
	if err := decode(w, r, &input); err != nil {
		s.fail(w, r, err)
		return
	}
	b, err := s.bookings.Confirm(r.Context(), r.PathValue("movieID"), input.SeatID, r.PathValue("bookingID"), currentUser(r).ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, b)
}

func (s *Server) release(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SeatID string `json:"seat_id"`
	}
	if err := decode(w, r, &input); err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.bookings.Release(r.Context(), r.PathValue("movieID"), input.SeatID, r.PathValue("bookingID"), currentUser(r).ID); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
