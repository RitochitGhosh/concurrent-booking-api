# Seat Booking API

A Go prototype for exploring a common booking problem: many users try to reserve the same movie seat at the same time, but only one booking should succeed.

This README explains the `02/concurrent-store` stage: an in-memory store using Go's `sync.RWMutex`. The original memory store remains available to illustrate why an ordinary map needs synchronization.

The current working tree also contains later Redis work. Its test currently selects `RedisStore`, and `main` starts an HTTP listener on port 8080 with no registered routes. The explanations below focus on `ConcurrentStore`.

## The problem: preventing double bookings

Checking whether a seat is available and saving its booking must happen as one atomic operation. If those steps can overlap between requests, two users can both see an available seat and both claim it.

For a single seat, the intended result is:

- Exactly one request succeeds.
- Every subsequent request receives `ErrSeatAlreadyBooked` (`seat is already taken`).

The current test launches 100,000 goroutines that all attempt to book seat `A1` for `screen-1`. It expects one success and 99,999 failures.

## What is implemented

| File | Purpose |
| --- | --- |
| `internal/booking/domain.go` | Defines `Booking`, `BookingStore`, and the duplicate-booking error. |
| `internal/booking/memory_Store.go` | Original in-memory store without synchronization. |
| `internal/booking/concurrent_Store.go` | In-memory store that protects bookings with an `RWMutex`. |
| `internal/booking/service.go` | Delegates booking requests to whichever store is supplied. |
| `internal/booking/service_test.go` | Tests concurrent attempts; the current working tree selects `RedisStore`. |
| `cmd/main.go` | Starts an HTTP listener on port 8080; no routes are registered. |

Both stores implement `BookingStore`, so callers can select the implementation when creating the service:

```go
store := booking.NewConcurrentStore()
svc := booking.NewService(store)

err := svc.Book(booking.Booking{
    MovieID: "screen-1",
    SeatID:  "A1",
    UserID:  "user-1",
})
```

This example assumes the caller has imported the internal `booking` package from within this module.

## Why the earlier memory store failed

`MemoryStore.Book` checks the map and then writes to it without a lock. Two goroutines can interleave like this:

| Step | Request A | Request B |
| --- | --- | --- |
| 1 | Checks `A1`: available | |
| 2 | | Checks `A1`: available |
| 3 | Saves its booking | |
| 4 | | Saves its booking |

This illustrates the missing atomicity: both requests could report success. In practice, Go maps also do not support unsynchronized access involving concurrent writes. The original test run crashed with `fatal error: concurrent map writes`. Reads during writes, including listing bookings, are unsafe too.

The test's atomic counters protect its success and failure counts. They do not protect the store's map or make the booking operation atomic.

## How the concurrent store uses locks

`ConcurrentStore` embeds a `sync.RWMutex` alongside its map. Every store method that accesses the map acquires the appropriate lock.

### Booking: exclusive access

```go
s.Lock()
defer s.Unlock()

if _, exists := s.bookings[b.SeatID]; exists {
    return ErrSeatAlreadyBooked
}

s.bookings[b.SeatID] = b
return nil
```

`Lock()` waits until exclusive access is available. While the lock is held, other calls to `Book` and `ListBookings` on this store must wait. The lock covers **both the availability check and the insertion**, so another request cannot slip between them.

After the first request saves the booking and releases the lock, the next request sees the occupied seat and returns the duplicate-booking error. `defer s.Unlock()` releases the lock when the method returns, including when it returns an error.

Locking only the insertion would not be sufficient: requests could still check availability simultaneously before either saved its booking.

### Listing: shared read access

`ListBookings` uses `RLock()` and `defer RUnlock()`. Multiple callers can read at the same time, but a reader cannot access the map while a writer holds the exclusive lock. A writer also waits for active readers to finish.

The read lock stays held throughout the map iteration, protecting the entire scan. Pointer receivers keep these methods operating on the same mutex and map; do not copy a `ConcurrentStore` after its mutex has been used.

## Pessimistic locking approach

This implementation uses **pessimistic locking**: it assumes requests may conflict and takes the lock before checking or changing shared state. Competing requests wait their turn, and each one sees the result of the previous booking.

An optimistic approach would allow work to proceed and detect conflicts when committing, for example with a database uniqueness constraint or a version check. The current implementation instead resolves contention using an exclusive lock around the booking operation.

The lock belongs to the whole store, so even bookings for different seats are serialized. This is straightforward and protects the current in-process data, but its guarantee applies only to callers sharing that same store instance.

## Remaining limitations

- **No persistence:** all bookings disappear when the process exits.
- **No coordination between instances:** separate stores or application processes have separate maps and locks. They can each accept the same seat. A shared database with an appropriate uniqueness constraint and atomic booking operation would be needed to enforce the rule across instances.
- **Seat identity is too broad:** the map key is only `SeatID`. Booking `A1` for one movie prevents booking `A1` for another movie. Separate showtimes need a key that identifies the showtime and seat.
- **One lock for all seats:** unrelated booking requests wait on the same mutex. Listing scans all bookings while holding a read lock, which can delay writers as the store grows.
- **No validation or booking lifecycle:** the code does not validate IDs, generate booking IDs or statuses, check that seats exist, or implement cancellation, temporary holds, expiry, or payment handling.
- **No booking HTTP routes yet:** the current listener does not connect requests to the booking service.
- **Limited test coverage:** the existing test counts successful and failed requests for one seat. It does not verify the exact error, the saved booking, concurrent listing, or behavior across movies or showtimes. A passing race check covers only the execution paths exercised by tests.

## How to test

Use the Go toolchain specified in `go.mod` (Go 1.26.7). To test this stage, select `store := NewConcurrentStore()` in `service_test.go` and remove or comment out the Redis client initialization and unused Redis adapter import. The current Redis-selected test does not exercise the mutex implementation. From the repository root:

```sh
go mod download
go test ./...
go test -race ./...
```

Run only the concurrent booking test (the spelling matches the current function name):

```sh
go test ./internal/booking -run '^TestConcurrentBookig_ExactlyOneWins$' -count=1 -v
```

With `NewConcurrentStore()` selected, the test should pass with one success and 99,999 failures. Switching it to `NewMemoryStore()` demonstrates the unsafe implementation; a run can crash or report races, and an occasional passing run does not prove safety.

The race detector requires a supported platform and a C compiler with cgo enabled. Launching 100,000 goroutines can consume substantial resources, especially with race detection.

To run the entry point:

```sh
go run ./cmd
```

The current entry point listens on port 8080. Requests receive HTTP 404 because no routes are registered.


## Publish the `02/concurrent-store` branch

Create a branch from the commit containing the in-memory concurrent-store stage. If that is your current commit, run:

```sh
git switch -c 02/concurrent-store
```

If the stage is on `01/memory-store` instead, use `git switch -c 02/concurrent-store 01/memory-store`. Check `git log --oneline --all` first to choose the right starting point. Switching branches may be blocked if your newer uncommitted changes conflict; preserve those changes before switching.

Once this branch contains the concurrent-store implementation and its test selects `NewConcurrentStore()`, run:

```sh
go test ./...
go test -race ./...
git add README.md internal/booking/concurrent_Store.go
git add -p internal/booking/service_test.go
git diff --cached
git commit -m "Document and test concurrent booking store with pessimistic locking"
git push -u origin 02/concurrent-store
```

Use interactive staging to include only the concurrent-store test changes. Review the staged diff before committing, and commit and push after the tests pass. If the implementation is already committed, only stage the remaining documentation and test changes. Keep the later Redis work out of this stage's commit. The `-u` option sets the upstream so future pushes can use `git push`.
