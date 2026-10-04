package httpapi

import (
	"net/http"
	"time"

	"github.com/RitochitGhosh/seatbooking-api/internal/auth"
	"github.com/RitochitGhosh/seatbooking-api/internal/platform"
)

func (s *Server) setTokens(w http.ResponseWriter, tokens auth.Tokens) {
	for _, c := range []struct {
		name, value, path string
		ttl               time.Duration
	}{
		{"access_token", tokens.Access, "/api", auth.AccessTTL},
		{"refresh_token", tokens.Refresh, "/api/auth", auth.RefreshTTL},
	} {
		http.SetCookie(w, &http.Cookie{Name: c.name, Value: c.value, Path: c.path, MaxAge: int(c.ttl.Seconds()), Expires: time.Now().Add(c.ttl), HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteStrictMode})
	}
}
func (s *Server) clearTokens(w http.ResponseWriter) {
	for name, path := range map[string]string{"access_token": "/api", "refresh_token": "/api/auth"} {
		http.SetCookie(w, &http.Cookie{Name: name, Path: path, MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteStrictMode})
	}
}
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decode(w, r, &input); err != nil {
		s.fail(w, r, err)
		return
	}
	u, tokens, err := s.auth.Register(r.Context(), input.Name, input.Email, input.Password)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.setTokens(w, tokens)
	writeJSON(w, 201, u)
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decode(w, r, &input); err != nil {
		s.fail(w, r, err)
		return
	}
	u, tokens, err := s.auth.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.setTokens(w, tokens)
	writeJSON(w, 200, u)
}
func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		s.fail(w, r, platform.ErrUnauthorized)
		return
	}
	u, tokens, err := s.auth.Refresh(r.Context(), cookie.Value)
	if err != nil {
		// A storage outage must not discard a valid refresh cookie.
		if err == platform.ErrUnauthorized {
			s.clearTokens(w)
		}
		s.fail(w, r, err)
		return
	}
	s.setTokens(w, tokens)
	writeJSON(w, 200, u)
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	for _, name := range []string{"refresh_token", "access_token"} {
		if cookie, err := r.Cookie(name); err == nil {
			if err := s.auth.Logout(r.Context(), cookie.Value); err != nil {
				s.fail(w, r, err)
				return
			}
		}
	}
	s.clearTokens(w)
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, currentUser(r)) }
