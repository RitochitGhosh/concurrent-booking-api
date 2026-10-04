package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RitochitGhosh/seatbooking-api/internal/auth"
	"github.com/RitochitGhosh/seatbooking-api/internal/booking"
	"github.com/RitochitGhosh/seatbooking-api/internal/movie"
	"github.com/RitochitGhosh/seatbooking-api/internal/testutil"
	"github.com/google/uuid"
)

func TestCSRFFailsBeforeHandlers(t *testing.T) {
	handler := New(nil, nil, nil, nil, nil, "http://localhost:3000", false, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, tc := range []struct{ origin, header string }{{"http://evil.test", "1"}, {"http://localhost:3000", ""}, {"", ""}} {
		r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{}`))
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("X-CSRF-Protection", tc.header)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("status = %d", w.Code)
		}
	}
}
func TestStrictJSON(t *testing.T) {
	for _, body := range []string{`{"email":"x","role":"ADMIN"}`, `{} {}`, `{"email":`, strings.Repeat("x", 17000)} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		var input struct {
			Email string `json:"email"`
		}
		if err := decode(httptest.NewRecorder(), r, &input); err == nil {
			t.Fatalf("accepted %q", body[:min(30, len(body))])
		}
	}
}

func TestAPIEndToEnd(t *testing.T) {
	db, rdb := testutil.Mongo(t), testutil.Redis(t)
	ctx := context.Background()
	users := auth.NewMongoStore(db)
	moviesStore := movie.NewMongoStore(db)
	bookingsStore := booking.NewMongoStore(db)
	for _, ensure := range []func(context.Context) error{users.EnsureIndexes, moviesStore.EnsureIndexes, bookingsStore.EnsureIndexes} {
		if err := ensure(ctx); err != nil {
			t.Fatal(err)
		}
	}
	a, err := auth.NewService(users, auth.NewRedisStore(rdb))
	if err != nil {
		t.Fatal(err)
	}
	adminEmail := uuid.NewString() + "@example.com"
	if err := a.BootstrapAdmin(ctx, adminEmail, "admin-password-123"); err != nil {
		t.Fatal(err)
	}
	m := movie.NewService(moviesStore)
	handler := New(a, m, booking.NewService(bookingsStore, m), rdb, func(context.Context) error { return nil }, "http://localhost:3000", false, slog.New(slog.NewTextHandler(io.Discard, nil)))
	type browser map[string]*http.Cookie
	admin, user, other := browser{}, browser{}, browser{}
	call := func(jar browser, method, path string, input any, want int) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(input)
		r := httptest.NewRequest(method, path, strings.NewReader(string(body)))
		r.RemoteAddr = "test-" + adminEmail + ":1234"
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Protection", "1")
		r.Header.Set("Origin", "http://localhost:3000")
		for _, cookie := range jar {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: status %d, want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		for _, cookie := range w.Result().Cookies() {
			if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
				t.Fatal("unsafe auth cookie")
			}
			if cookie.MaxAge < 0 {
				delete(jar, cookie.Name)
			} else {
				jar[cookie.Name] = cookie
			}
		}
		return w
	}
	call(user, "GET", "/api/movies", nil, 401)
	call(user, "POST", "/api/auth/register", map[string]string{"name": "User", "email": uuid.NewString() + "@example.com", "password": "user-password-123", "role": "ADMIN"}, 400)
	registration := call(user, "POST", "/api/auth/register", map[string]string{"name": "User", "email": uuid.NewString() + "@example.com", "password": "user-password-123"}, 201)
	if strings.Contains(registration.Body.String(), "password") {
		t.Fatal("password leaked")
	}
	call(other, "POST", "/api/auth/register", map[string]string{"name": "Other", "email": uuid.NewString() + "@example.com", "password": "user-password-123"}, 201)
	call(admin, "POST", "/api/auth/login", map[string]string{"email": adminEmail, "password": "admin-password-123"}, 200)
	input := map[string]any{"title": "A Test Story", "poster_url": "https://example.com/poster.jpg", "genre": "Drama", "duration_minutes": 120, "starts_at": time.Now().Add(time.Hour), "rows": 6, "seats_per_row": 10}
	call(user, "POST", "/api/movies", input, 403)
	w := call(admin, "POST", "/api/movies", input, 201)
	var screening movie.Movie
	if err := json.Unmarshal(w.Body.Bytes(), &screening); err != nil {
		t.Fatal(err)
	}
	if screening.PosterURL != "https://example.com/poster.jpg" {
		t.Fatal("poster URL missing")
	}
	call(user, "GET", "/api/movies", nil, 200)
	path := "/api/movies/" + screening.ID
	call(user, "POST", path+"/bookings", map[string]string{"seat_id": "Z1"}, 400)
	w = call(user, "POST", path+"/bookings", map[string]string{"seat_id": "A1"}, 201)
	var b booking.Booking
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	call(other, "POST", path+"/bookings", map[string]string{"seat_id": "A1"}, 409)
	availability := call(other, "GET", path+"/seats", nil, 200)
	if strings.Contains(availability.Body.String(), b.UserID) || strings.Contains(availability.Body.String(), b.ID) {
		t.Fatal("other user's booking leaked")
	}
	confirm := path + "/bookings/" + b.ID + "/confirm"
	call(other, "POST", confirm, map[string]string{"seat_id": "A1"}, 403)
	call(user, "POST", confirm, map[string]string{"seat_id": "A1"}, 200)
	call(user, "POST", confirm, map[string]string{"seat_id": "A1"}, 200)
	call(user, "DELETE", path+"/bookings/"+b.ID, map[string]string{"seat_id": "A1"}, 409)
	w = call(user, "POST", path+"/bookings", map[string]string{"seat_id": "A2"}, 201)
	var held booking.Booking
	if err := json.Unmarshal(w.Body.Bytes(), &held); err != nil {
		t.Fatal(err)
	}
	release := path + "/bookings/" + held.ID
	call(other, "DELETE", release, map[string]string{"seat_id": "A2"}, 403)
	call(user, "DELETE", release, map[string]string{"seat_id": "A2"}, 204)
	call(user, "DELETE", release, map[string]string{"seat_id": "A2"}, 204)
	call(other, "POST", path+"/bookings", map[string]string{"seat_id": "A2"}, 201)
	oldAccess := *user["access_token"]
	oldRefresh := *user["refresh_token"]
	call(user, "POST", "/api/auth/refresh", nil, 200)
	call(browser{"access_token": &oldAccess}, "GET", "/api/auth/me", nil, 401)
	call(browser{"refresh_token": &oldRefresh}, "POST", "/api/auth/refresh", nil, 401)
	call(user, "GET", "/api/auth/me", nil, 401)
	call(admin, "POST", "/api/auth/logout", nil, 204)
	call(admin, "GET", "/api/auth/me", nil, 401)
	call(other, "POST", "/api/auth/logout", nil, 204)
}
