package booking

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/RitochitGhosh/seatbooking-api/internal/platform"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RedisStore struct{ rdb *redis.Client }

func NewRedisStore(rdb *redis.Client) *RedisStore { return &RedisStore{rdb: rdb} }

// The hash tag keeps each screening's keys together if moving to Redis Cluster.
func seatKey(movieID, seatID string) string {
	return fmt.Sprintf("booking:{%s}:seat:%s", movieID, seatID)
}
func indexKey(movieID string) string { return fmt.Sprintf("booking:{%s}:seats", movieID) }

// Lua commits the hold and its index together. Expired keys are ignored on listing.
var holdScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 1 then return 0 end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
redis.call('SADD', KEYS[2], KEYS[1])
return 1
`)

func (s *RedisStore) Hold(ctx context.Context, b Booking) (Booking, error) {
	b.ID, b.Status, b.ExpiresAt = uuid.NewString(), "held", time.Now().UTC().Add(defaultHoldTTL)
	data, err := json.Marshal(b)
	if err != nil {
		return Booking{}, err
	}
	result, err := holdScript.Run(ctx, s.rdb, []string{seatKey(b.MovieID, b.SeatID), indexKey(b.MovieID)}, data, defaultHoldTTL.Milliseconds()).Int()
	if err != nil {
		return Booking{}, err
	}
	if result == 0 {
		return Booking{}, ErrSeatAlreadyBooked
	}
	return b, nil
}

// Ownership, hold identity, and permanence are checked/changed atomically.
// Repeating a confirmation of the same reservation is safe.
var confirmScript = redis.NewScript(`
local raw = redis.call('GET', KEYS[1])
if not raw then return {0, ''} end
local b = cjson.decode(raw)
if b.id ~= ARGV[1] then return {0, ''} end
if b.user_id ~= ARGV[2] then return {2, ''} end
b.status = 'confirmed'
b.expires_at = '0001-01-01T00:00:00Z'
local result = cjson.encode(b)
redis.call('SET', KEYS[1], result)
return {1, result}
`)

func (s *RedisStore) Confirm(ctx context.Context, movieID, seatID, id, userID string) (Booking, error) {
	result, err := confirmScript.Run(ctx, s.rdb, []string{seatKey(movieID, seatID)}, id, userID).Slice()
	if err != nil {
		return Booking{}, err
	}
	switch result[0].(int64) {
	case 0:
		return Booking{}, platform.ErrNotFound
	case 2:
		return Booking{}, platform.ErrForbidden
	}
	var b Booking
	err = json.Unmarshal([]byte(result[1].(string)), &b)
	return b, err
}

var releaseScript = redis.NewScript(`
local raw = redis.call('GET', KEYS[1])
if not raw then return 0 end
local b = cjson.decode(raw)
if b.id ~= ARGV[1] then return 0 end
if b.user_id ~= ARGV[2] then return 2 end
if b.status ~= 'held' then return 3 end
redis.call('DEL', KEYS[1])
redis.call('SREM', KEYS[2], KEYS[1])
return 1
`)

func (s *RedisStore) Release(ctx context.Context, movieID, seatID, id, userID string) error {
	result, err := releaseScript.Run(ctx, s.rdb, []string{seatKey(movieID, seatID), indexKey(movieID)}, id, userID).Int()
	if err != nil {
		return err
	}
	if result == 2 {
		return platform.ErrForbidden
	}
	if result == 3 {
		return ErrHoldNotActive
	}
	return nil
}

func (s *RedisStore) List(ctx context.Context, movieID string) ([]Booking, error) {
	keys, err := s.rdb.SMembers(ctx, indexKey(movieID)).Result()
	if err != nil {
		return nil, err
	}
	bookings := make([]Booking, 0, len(keys))
	if len(keys) == 0 {
		return bookings, nil
	}
	values, err := s.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		if value == nil {
			continue
		}
		var b Booking
		if err := json.Unmarshal([]byte(value.(string)), &b); err != nil {
			return nil, err
		}
		bookings = append(bookings, b)
	}
	return bookings, nil
}
