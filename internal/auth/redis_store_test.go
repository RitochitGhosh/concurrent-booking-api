package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RitochitGhosh/seatbooking-api/internal/platform"
	"github.com/RitochitGhosh/seatbooking-api/internal/testutil"
	"github.com/google/uuid"
)

func TestSessionRotationAndRevocation(t *testing.T) {
	rdb := testutil.Redis(t)
	s := NewRedisStore(rdb)
	ctx := context.Background()
	id := uuid.NewString()
	t.Cleanup(func() { rdb.Del(ctx, sessionKey(id), usedKey(id)) })
	if err := s.CreateSession(ctx, id, "user", "a1", "r1"); err != nil {
		t.Fatal(err)
	}
	if user, err := s.AccessUser(ctx, id, "a1"); err != nil || user != "user" {
		t.Fatalf("access: %q, %v", user, err)
	}
	if _, err := s.Rotate(ctx, id, "bogus", "a2", "r2"); !errors.Is(err, platform.ErrUnauthorized) {
		t.Fatal("invalid refresh accepted")
	}
	if _, err := s.AccessUser(ctx, id, "a1"); err != nil {
		t.Fatal("random refresh revoked session")
	}
	if _, err := s.Rotate(ctx, id, "r1", "a2", "r2"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AccessUser(ctx, id, "a1"); !errors.Is(err, platform.ErrUnauthorized) {
		t.Fatal("old access accepted")
	}
	if _, err := s.Rotate(ctx, id, "r1", "a3", "r3"); !errors.Is(err, platform.ErrUnauthorized) {
		t.Fatal("replayed refresh accepted")
	}
	if _, err := s.AccessUser(ctx, id, "a2"); !errors.Is(err, platform.ErrUnauthorized) {
		t.Fatal("replay did not revoke family")
	}
	if err := s.CreateSession(ctx, id, "user", "a4", "r4"); err != nil {
		t.Fatal(err)
	}
	rdb.HSet(ctx, sessionKey(id), "access_until", time.Now().Add(-time.Second).UnixMilli())
	if _, err := s.AccessUser(ctx, id, "a4"); !errors.Is(err, platform.ErrUnauthorized) {
		t.Fatal("expired access accepted")
	}
	if _, err := s.Rotate(ctx, id, "r4", "a5", "r5"); err != nil {
		t.Fatal("expired access prevented refresh")
	}
	if err := s.Revoke(ctx, id, "r5"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AccessUser(ctx, id, "a5"); !errors.Is(err, platform.ErrUnauthorized) {
		t.Fatal("logout did not revoke access")
	}
}

type unavailableUsers struct{}

func (unavailableUsers) CreateUser(context.Context, Credentials) error {
	return errors.New("database unavailable")
}
func (unavailableUsers) UserByEmail(context.Context, string) (Credentials, error) {
	return Credentials{}, errors.New("database unavailable")
}
func (unavailableUsers) UserByID(context.Context, string) (User, error) {
	return User{}, errors.New("database unavailable")
}
func TestUserLookupFailureDoesNotConsumeRefresh(t *testing.T) {
	rdb := testutil.Redis(t)
	sessions := NewRedisStore(rdb)
	service, err := NewService(unavailableUsers{}, sessions)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	tokens, err := service.newSession(ctx, "user")
	if err != nil {
		t.Fatal(err)
	}
	id, hash, err := tokenParts(tokens.Refresh)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rdb.Del(ctx, sessionKey(id), usedKey(id)) })
	if _, _, err := service.Refresh(ctx, tokens.Refresh); err == nil {
		t.Fatal("database outage ignored")
	}
	if _, err := sessions.Rotate(ctx, id, hash, "new-access", "new-refresh"); err != nil {
		t.Fatalf("valid refresh consumed during outage: %v", err)
	}
}
