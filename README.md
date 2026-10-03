# Seat Booking API

A Go prototype for reserving movie seats and exploring how to prevent double bookings when many users try to reserve the same seat at once.

## Problem we are trying to solve

When multiple users request the same seat, checking availability and saving a booking must happen as one atomic operation. Otherwise, multiple requests can see the seat as available and each claim it.

The intended behavior is that exactly one request succeeds and every other request for that seat receives `ErrSeatAlreadyBooked` (`seat is already taken`). The existing test exercises this with 100,000 goroutines attempting to book seat `A1` for `screen-1`.

## Current implementation

- `internal/booking/domain.go` defines `Booking`, the `BookingStore` interface, and the duplicate-booking error.
- `internal/booking/memory_Store.go` implements an in-memory store, rejects an already stored seat, and lists bookings by movie ID.
- `internal/booking/service.go` provides a service that delegates booking requests to the store.
- `internal/booking/service_test.go` contains the concurrent booking test.
- `cmd/main.go` contains an empty entry point. There are currently no HTTP routes or running API server.

Bookings contain an ID, movie ID, seat ID, user ID, and status. The current code does not validate these fields or generate booking IDs or statuses. Data is kept only in memory and is lost when the process exits.

### Known limitations

The store uses a Go map without synchronization. Concurrent booking requests can race and crash with `fatal error: concurrent map writes` or concurrent map reads and writes. The check for an existing seat and the insertion are not atomic, so the intended concurrency guarantee is not yet implemented. Listing bookings while writing to the store is also unsafe.

Bookings are keyed only by `SeatID`. Consequently, booking `A1` for one movie also prevents booking `A1` for another movie. Supporting separate movies or showtimes requires an appropriate reservation key.

## How to test

Use the Go toolchain specified in `go.mod` (Go 1.26.7). From the repository root, download dependencies and run all tests:

```sh
go mod download
go test ./...
```

Run the concurrent booking test alone (the spelling below matches the current test name):

```sh
go test ./internal/booking -run '^TestConcurrentBookig_ExactlyOneWins$' -count=1 -v
```

Use the race detector to check concurrent access:

```sh
go test -race ./...
```

The race detector requires a supported platform and a C compiler with cgo enabled. The concurrency test starts 100,000 goroutines, so it can consume substantial resources, especially with race detection enabled.

The test expects one successful booking and 99,999 failed bookings. **The current implementation is expected to fail under concurrent access**; a test run during README preparation failed with `fatal error: concurrent map writes`. A passing run without race detection does not establish that the store is safe.

## Running the entry point

```sh
go run ./cmd
```

This currently exits without starting a server or producing output because `main` is empty.
