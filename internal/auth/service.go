package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/RitochitGhosh/seatbooking-api/internal/platform"
	"github.com/google/uuid"
)

type Service struct {
	users     UserStore
	sessions  SessionStore
	dummyHash string
}

func NewService(users UserStore, sessions SessionStore) (*Service, error) {
	// A missing account still performs the same expensive password check.
	hash, err := hashPassword(uuid.NewString())
	if err != nil {
		return nil, err
	}
	return &Service{users: users, sessions: sessions, dummyHash: hash}, nil
}
func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 {
		return "", platform.ErrInvalid
	}
	return email, nil
}
func (s *Service) createUser(ctx context.Context, name, email, password, role string) (User, error) {
	email, err := normalizeEmail(email)
	name = strings.TrimSpace(name)
	if err != nil || name == "" || len(name) > 80 || len(password) < 12 || len(password) > 128 {
		return User{}, fmt.Errorf("%w: name, valid email and a password of 12–128 bytes are required", platform.ErrInvalid)
	}
	hash, err := hashPassword(password)
	if err != nil {
		return User{}, err
	}
	u := User{ID: uuid.NewString(), Name: name, Email: email, Role: role}
	return u, s.users.CreateUser(ctx, Credentials{User: u, PasswordHash: hash})
}
func (s *Service) Register(ctx context.Context, name, email, password string) (User, Tokens, error) {
	u, err := s.createUser(ctx, name, email, password, RoleUser)
	if err != nil {
		return User{}, Tokens{}, err
	}
	tokens, err := s.newSession(ctx, u.ID)
	return u, tokens, err
}
func (s *Service) BootstrapAdmin(ctx context.Context, email, password string) error {
	email, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	credentials, err := s.users.UserByEmail(ctx, email)
	if err == nil {
		if credentials.User.Role != RoleAdmin {
			return fmt.Errorf("bootstrap email already belongs to a non-admin account")
		}
		return nil // Never silently promote an account or reset its password.
	}
	if !errors.Is(err, platform.ErrNotFound) {
		return err
	}
	_, err = s.createUser(ctx, "Administrator", email, password, RoleAdmin)
	return err
}
func (s *Service) Login(ctx context.Context, email, password string) (User, Tokens, error) {
	email, err := normalizeEmail(email)
	if err != nil || len(password) > 128 {
		return User{}, Tokens{}, platform.ErrUnauthorized
	}
	c, err := s.users.UserByEmail(ctx, email)
	if err != nil && !errors.Is(err, platform.ErrNotFound) {
		return User{}, Tokens{}, err
	}
	hash := c.PasswordHash
	if err != nil {
		hash = s.dummyHash
	}
	if !checkPassword(hash, password) || err != nil {
		return User{}, Tokens{}, platform.ErrUnauthorized
	}
	tokens, err := s.newSession(ctx, c.User.ID)
	return c.User, tokens, err
}
func token(sessionID string) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return sessionID + "." + hex.EncodeToString(bytes), nil
}
func tokenParts(value string) (string, string, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 || len(parts[0]) != 36 || len(parts[1]) != 64 {
		return "", "", platform.ErrUnauthorized
	}
	if _, err := uuid.Parse(parts[0]); err != nil {
		return "", "", platform.ErrUnauthorized
	}
	if _, err := hex.DecodeString(parts[1]); err != nil {
		return "", "", platform.ErrUnauthorized
	}
	hash := sha256.Sum256([]byte(value))
	return parts[0], hex.EncodeToString(hash[:]), nil
}
func newTokens(id string) (Tokens, error) {
	access, err := token(id)
	if err != nil {
		return Tokens{}, err
	}
	refresh, err := token(id)
	return Tokens{Access: access, Refresh: refresh}, err
}
func (s *Service) newSession(ctx context.Context, userID string) (Tokens, error) {
	id := uuid.NewString()
	tokens, err := newTokens(id)
	if err != nil {
		return Tokens{}, err
	}
	_, a, _ := tokenParts(tokens.Access)
	_, r, _ := tokenParts(tokens.Refresh)
	return tokens, s.sessions.CreateSession(ctx, id, userID, a, r)
}
func (s *Service) Authenticate(ctx context.Context, access string) (User, error) {
	id, hash, err := tokenParts(access)
	if err != nil {
		return User{}, err
	}
	userID, err := s.sessions.AccessUser(ctx, id, hash)
	if err != nil {
		return User{}, err
	}
	u, err := s.users.UserByID(ctx, userID)
	if errors.Is(err, platform.ErrNotFound) {
		return User{}, platform.ErrUnauthorized
	}
	return u, err
}
func (s *Service) Refresh(ctx context.Context, refresh string) (User, Tokens, error) {
	id, old, err := tokenParts(refresh)
	if err != nil {
		return User{}, Tokens{}, err
	}
	// Read the user before rotation: a MongoDB outage must not consume the
	// browser's refresh token. No credentials are returned until Rotate succeeds.
	userID, err := s.sessions.SessionUser(ctx, id)
	if err != nil {
		return User{}, Tokens{}, err
	}
	u, err := s.users.UserByID(ctx, userID)
	if errors.Is(err, platform.ErrNotFound) {
		return User{}, Tokens{}, platform.ErrUnauthorized
	}
	if err != nil {
		return User{}, Tokens{}, err
	}
	tokens, err := newTokens(id)
	if err != nil {
		return User{}, Tokens{}, err
	}
	_, a, _ := tokenParts(tokens.Access)
	_, r, _ := tokenParts(tokens.Refresh)
	if _, err := s.sessions.Rotate(ctx, id, old, a, r); err != nil {
		return User{}, Tokens{}, err
	}
	return u, tokens, nil
}
func (s *Service) Logout(ctx context.Context, value string) error {
	id, hash, err := tokenParts(value)
	if err != nil {
		return nil
	}
	return s.sessions.Revoke(ctx, id, hash)
}
