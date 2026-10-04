package auth

import (
	"context"
	"errors"
	"time"

	"github.com/RitochitGhosh/seatbooking-api/internal/platform"
	"github.com/redis/go-redis/v9"
)

type RedisStore struct{ rdb *redis.Client }

func NewRedisStore(rdb *redis.Client) *RedisStore { return &RedisStore{rdb: rdb} }

func sessionKey(id string) string { return "auth:{" + id + "}:session" }
func usedKey(id string) string    { return "auth:{" + id + "}:used" }

var createSessionScript = redis.NewScript(`
redis.call('HSET', KEYS[1], 'user', ARGV[1], 'access', ARGV[2], 'refresh', ARGV[3], 'access_until', ARGV[4])
redis.call('PEXPIRE', KEYS[1], ARGV[5])
return 1
`)

func (s *RedisStore) CreateSession(ctx context.Context, id, user, access, refresh string) error {
	return createSessionScript.Run(ctx, s.rdb, []string{sessionKey(id)}, user, access, refresh, time.Now().Add(AccessTTL).UnixMilli(), RefreshTTL.Milliseconds()).Err()
}

// This lookup is not authentication. Rotate still validates the refresh hash.
func (s *RedisStore) SessionUser(ctx context.Context, id string) (string, error) {
	user, err := s.rdb.HGet(ctx, sessionKey(id), "user").Result()
	if errors.Is(err, redis.Nil) {
		return "", platform.ErrUnauthorized
	}
	return user, err
}

var accessScript = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'access') ~= ARGV[1] then return '' end
local until_at = tonumber(redis.call('HGET', KEYS[1], 'access_until') or '0')
if until_at <= tonumber(ARGV[2]) then return '' end
return redis.call('HGET', KEYS[1], 'user') or ''
`)

func (s *RedisStore) AccessUser(ctx context.Context, id, hash string) (string, error) {
	user, err := accessScript.Run(ctx, s.rdb, []string{sessionKey(id)}, hash, time.Now().UnixMilli()).Text()
	if err == nil && user == "" {
		return "", platform.ErrUnauthorized
	}
	return user, err
}

// A known, already-used refresh token revokes the entire session family.
// Random invalid tokens cannot revoke a session. The absolute lifetime stays fixed.
var rotateScript = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'refresh') ~= ARGV[1] then
  if redis.call('SISMEMBER', KEYS[2], ARGV[1]) == 1 then redis.call('DEL', KEYS[1], KEYS[2]) end
  return ''
end
local ttl = redis.call('PTTL', KEYS[1])
if ttl <= 0 then return '' end
local user = redis.call('HGET', KEYS[1], 'user')
redis.call('SADD', KEYS[2], ARGV[1])
redis.call('PEXPIRE', KEYS[2], ttl)
redis.call('HSET', KEYS[1], 'access', ARGV[2], 'refresh', ARGV[3], 'access_until', ARGV[4])
return user
`)

func (s *RedisStore) Rotate(ctx context.Context, id, old, access, refresh string) (string, error) {
	user, err := rotateScript.Run(ctx, s.rdb, []string{sessionKey(id), usedKey(id)}, old, access, refresh, time.Now().Add(AccessTTL).UnixMilli()).Text()
	if err == nil && user == "" {
		return "", platform.ErrUnauthorized
	}
	return user, err
}

var revokeScript = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'access') == ARGV[1] or redis.call('HGET', KEYS[1], 'refresh') == ARGV[1] or redis.call('SISMEMBER', KEYS[2], ARGV[1]) == 1 then
  redis.call('DEL', KEYS[1], KEYS[2])
end
return 1
`)

func (s *RedisStore) Revoke(ctx context.Context, id, hash string) error {
	return revokeScript.Run(ctx, s.rdb, []string{sessionKey(id), usedKey(id)}, hash).Err()
}
