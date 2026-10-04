# Seat Booking API

A Go prototype for preventing double bookings when many users try to reserve the same movie seat. The current `RedisStore` creates temporary seat holds in shared Redis storage.

## Architecture

```text
Caller → booking.Service → BookingStore → RedisStore → Redis
```

- **Domain (`internal/booking/domain.go`):** defines `Booking`, the `BookingStore` interface, and `ErrSeatAlreadyBooked`.
- **Service (`internal/booking/service.go`):** delegates booking requests to the injected store.
- **Redis store (`internal/booking/redis_Store.go`):** creates a session UUID and claims a seat using an atomic Redis write with a two-minute expiry.
- **Redis adapter (`internal/adapters/redis/redi.go`):** creates the Redis client and checks connectivity with `PING`.
- **Entry point (`cmd/main.go`):** starts an HTTP listener on port 8080. Booking routes and service wiring are not implemented yet; the test wires the service to Redis.

`MemoryStore` and `ConcurrentStore` are earlier implementations of the same interface. `MemoryStore` uses an unsynchronized map; `ConcurrentStore` protects its map with `sync.RWMutex`. Redis lets separate application instances coordinate seat holds when they share the same Redis server.

## How RedisStore works

| Redis key | Value | Expiry |
| --- | --- | --- |
| `seat:{movieID}:{seatID}` | Booking JSON with a generated session ID | Two minutes |
| `session:{sessionID}` | The corresponding seat key | Two minutes |

1. `Service.Book` calls `RedisStore.Book`, which creates a temporary hold.
2. The store generates a UUID, adds it to the booking, and serializes the booking to JSON.
3. Redis executes `SET seat:{movieID}:{seatID} <json> NX` with a two-minute TTL. `NX` only creates the key if it does not already exist, so checking availability and claiming the seat happen atomically.
4. The winner creates the session reverse-lookup key and logs the hold. Competing requests receive `ErrSeatAlreadyBooked` while the seat key exists.
5. When the seat key expires, the seat becomes available again.

The seat key includes both movie and seat IDs, so `A1` for different movies can be held independently. Separate showtimes would need their own identity in the key.

## Current limitations

- `Book` returns only an error; the generated session is logged rather than returned to the caller. Confirmation, cancellation, and payment handling are not implemented.
- `ListBookings` currently returns an empty slice.
- The hold result includes `Status: "held"` and `ExpiresAt`, but those fields are populated after serialization, so the stored JSON does not receive these values automatically.
- Seat and session keys are written separately, without a transaction. Redis command errors are not propagated correctly: failed seat writes are reported as duplicate bookings, and session-write errors are ignored.
- The Compose setup has no configured Redis data volume, and the store does not validate IDs or check whether a seat exists.

## Run locally

Use the Go toolchain specified in `go.mod` (Go 1.26.7) and Docker Compose.

```sh
docker compose up -d
go mod download
go run ./cmd
```

Redis is exposed at `localhost:6380`. Redis Commander is available at `http://localhost:8081`. The HTTP listener runs at `http://localhost:8080` and returns 404 until routes are added.

## Test concurrent booking

With Redis running:

```sh
go test ./internal/booking -run '^TestConcurrentBookig_ExactlyOneWins$' -count=1 -v
```

The test launches 100,000 goroutines against `screen-1`, seat `A1`, and expects one success and 99,999 failures. Start with that seat key absent; an existing hold can cause all attempts to fail. Wait for its two-minute expiry before repeating the test. The test also assumes all attempts finish before the winning hold expires.

To run all tests or enable Go's race detector:

```sh
go test ./...
go test -race ./...
```

These tests currently require Redis at `localhost:6380`. The race detector requires a supported platform and a C compiler with cgo enabled; the 100,000-goroutine test can consume substantial resources.
