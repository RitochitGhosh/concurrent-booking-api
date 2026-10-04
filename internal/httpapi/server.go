package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/RitochitGhosh/seatbooking-api/internal/auth"
	"github.com/RitochitGhosh/seatbooking-api/internal/booking"
	"github.com/RitochitGhosh/seatbooking-api/internal/movie"
	"github.com/RitochitGhosh/seatbooking-api/internal/platform"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	auth        *auth.Service
	movies      *movie.Service
	bookings    *booking.Service
	rdb         *redis.Client
	mongoHealth func(context.Context) error
	origin      string
	secure      bool
	log         *slog.Logger
}

func New(a *auth.Service, m *movie.Service, b *booking.Service, rdb *redis.Client, mongoHealth func(context.Context) error, origin string, secure bool, logger *slog.Logger) http.Handler {
	s := &Server{auth: a, movies: m, bookings: b, rdb: rdb, mongoHealth: mongoHealth, origin: origin, secure: secure, log: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /api/auth/register", s.rateLimit(s.register))
	mux.HandleFunc("POST /api/auth/login", s.rateLimit(s.login))
	mux.HandleFunc("POST /api/auth/refresh", s.rateLimit(s.refresh))
	mux.HandleFunc("POST /api/auth/logout", s.logout)
	mux.HandleFunc("GET /api/auth/me", s.requireUser(s.me, false))
	mux.HandleFunc("GET /api/movies", s.requireUser(s.listMovies, false))
	mux.HandleFunc("POST /api/movies", s.requireUser(s.createMovie, true))
	mux.HandleFunc("GET /api/movies/{movieID}/seats", s.requireUser(s.listSeats, false))
	mux.HandleFunc("POST /api/movies/{movieID}/bookings", s.requireUser(s.hold, false))
	mux.HandleFunc("POST /api/movies/{movieID}/bookings/{bookingID}/confirm", s.requireUser(s.confirm, false))
	mux.HandleFunc("DELETE /api/movies/{movieID}/bookings/{bookingID}", s.requireUser(s.release, false))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { s.fail(w, r, platform.ErrNotFound) })
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { s.fail(w, r, platform.ErrNotFound) })
	return s.middleware(mux)
}

type userKey struct{}

func currentUser(r *http.Request) auth.User { return r.Context().Value(userKey{}).(auth.User) }

func (s *Server) requireUser(next http.HandlerFunc, admin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("access_token")
		if err != nil {
			s.fail(w, r, platform.ErrUnauthorized)
			return
		}
		u, err := s.auth.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if admin && u.Role != auth.RoleAdmin {
			s.fail(w, r, platform.ErrForbidden)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userKey{}, u)))
	}
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.NewString()
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' https://images.unsplash.com; style-src 'self'; script-src 'self'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		if s.secure {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		defer func() {
			if value := recover(); value != nil {
				s.log.Error("request panic", "request_id", requestID, "panic", value)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			}
		}()
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
			// Custom headers cannot be sent by HTML forms or cross-origin scripts
			// without a successful CORS preflight. This API intentionally has no CORS.
			if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
				origin := r.Header.Get("Origin")
				if r.Header.Get("X-CSRF-Protection") != "1" || (origin != "" && origin != s.origin) || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
					s.fail(w, r, platform.ErrForbidden)
					return
				}
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

var rateScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('EXPIRE', KEYS[1], 60) end
return n
`)

func (s *Server) rateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		// Do not trust client-controlled X-Forwarded-For. Configure proxy trust
		// explicitly before adapting this for a reverse proxy deployment.
		n, err := rateScript.Run(r.Context(), s.rdb, []string{"rate:auth:" + ip}).Int()
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if n > 20 {
			w.Header().Set("Retry-After", "60")
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts; try again in a minute"})
			return
		}
		next(w, r)
	}
}

func decode(w http.ResponseWriter, r *http.Request, value any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return platform.ErrInvalid
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return platform.ErrInvalid
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return platform.ErrInvalid
	}
	return nil
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	status, message := http.StatusInternalServerError, "internal server error"
	switch {
	case errors.Is(err, platform.ErrInvalid):
		status, message = 400, err.Error()
	case errors.Is(err, platform.ErrUnauthorized):
		status, message = 401, "sign in to continue"
	case errors.Is(err, platform.ErrForbidden):
		status, message = 403, "permission denied"
	case errors.Is(err, platform.ErrNotFound):
		status, message = 404, "not found or hold expired"
	case errors.Is(err, platform.ErrConflict):
		status, message = 409, "already exists"
	case errors.Is(err, booking.ErrHoldNotActive):
		status, message = 409, err.Error()
	case errors.Is(err, booking.ErrSeatAlreadyBooked):
		status, message = 409, "seat is already taken"
	default:
		s.log.Error("request failed", "request_id", w.Header().Get("X-Request-ID"), "method", r.Method, "path", r.URL.Path, "error", err)
	}
	writeJSON(w, status, map[string]string{"error": message})
}
