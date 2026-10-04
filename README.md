# Seat Booking API · FRAME

A small Go web API with a Next.js cinema frontend. Admins add screenings; users browse movies, hold a seat for two minutes, and explicitly confirm their reservation. No payment integration.

## Architecture

```text
Browser → Next.js (localhost:3000)
               │ /api/* proxy, same-origin cookies
               ▼
          Go net/http (localhost:8080)
               │ handlers → services → store interfaces
               ├── MongoDB: users, screenings, seat reservations
               └── Redis: access/refresh sessions and auth rate limits
```

Go owns authentication, authorization, validation, and booking rules. Next.js handles presentation and proxies API requests; it never receives MongoDB credentials. Server wiring lives in `cmd/main.go`, with small feature packages rather than a framework or a generic repository layer.

| Location | Responsibility |
| --- | --- |
| `cmd/main.go` | Load config, connect databases, ensure indexes, wire services, start/shut down HTTP. |
| `internal/config` | Read root `.env` and environment variables; validate application origin. |
| `internal/httpapi` | Routing, strict JSON input, auth/RBAC middleware, CSRF checks, status codes, cookie handling. |
| `internal/auth` | Password hashing, user registration/login, access tokens, refresh rotation, revocation. |
| `internal/movie` | Screening validation, catalog service, MongoDB store. |
| `internal/booking` | Seat validation, two-minute holds, confirmation, MongoDB store. |
| `internal/adapters` | MongoDB and Redis connection setup. |
| `frontend/app` | Next.js App Router page, layout, responsive cinema styles. |
| `frontend/components` | Accessible dialogs for authentication, admin creation, and seat selection; shadcn calendar/time controls. |
| `frontend/lib` | Typed API client, session refresh, shared response types. |

Store interfaces sit beside the services that use them. Handlers translate HTTP input into service calls, and stores handle database operations. Request contexts reach database calls so disconnects and deadlines can cancel work.

## Why MongoDB owns reservations

A movie currently represents **one screening**, including its time and seating layout. Seat identity is `(movie_id, seat_id)`, such as `(screening UUID, A1)`.

MongoDB has a unique compound index on this pair. A reservation is one document with either `held` or `confirmed` status:

```text
available → held (2 minutes) → confirmed
               │
               └── expires → available for another hold
```

- **Hold:** `FindOneAndUpdate` reclaims an expired hold or upserts a free seat. A competing request for an active/confirmed seat hits the unique index and receives HTTP 409. Each new hold gets a fresh booking UUID.
- **Expiry:** a hold is expired as soon as `expires_at` passes. Availability filters it out, and the next hold atomically replaces it. Correctness does not depend on MongoDB's delayed TTL cleanup.
- **Confirm:** one atomic update matches screening, seat, booking ID, owner, `held` status, and an unexpired deadline. It sets `confirmed` and removes expiry. Another user's confirmation is rejected.
- **Release:** the owner can delete a held seat immediately. The atomic delete matches its ID, owner, and `held` status, so it cannot remove a confirmation or a newer hold. Repeated release requests are safe.
- **Retry:** confirming the same owned reservation again returns the confirmed booking. This handles a lost confirmation response without creating another booking.

There is no Redis/MongoDB distributed transaction: the complete reservation transition happens in MongoDB. Its unique index is the final defense against double booking across API instances. The application requests majority-acknowledged writes; database errors never become successful bookings.

The original `MemoryStore`, `ConcurrentStore`, and `RedisStore` remain as learning examples. The deployed API uses `MongoStore`. The Redis example uses atomic `SET NX` holds and Lua confirmation, but it is not the primary reservation database.

## Authentication and access control

- Public registration always creates a `USER`. Clients cannot choose their role.
- Bootstrap an `ADMIN` with `ADMIN_EMAIL` and `ADMIN_PASSWORD` on first startup. An existing user is never silently promoted, and an existing admin password is not reset on restart. Remove bootstrap variables after creating the admin.
- Passwords use salted PBKDF2-HMAC-SHA256 with 600,000 iterations. Password hashes are never included in JSON responses.
- Access tokens last 15 minutes. Refresh sessions have a fixed seven-day absolute lifetime. Both tokens are opaque random secrets; Redis stores hashes, not plaintext tokens.
- Cookies are HttpOnly, SameSite=Strict, and Secure when `APP_ORIGIN` uses HTTPS. Access cookies cover `/api`; refresh cookies cover `/api/auth`.
- Refresh atomically replaces both tokens. A replay of a previously used refresh token revokes that session family. Logout revokes it immediately, including its access token.
- The frontend retries an authenticated request once after refresh and coordinates refreshes within a tab and across tabs with Web Locks where available. No token is stored in localStorage.
- API mutations require `X-CSRF-Protection: 1`. Browser origins must match `APP_ORIGIN`, and cross-site requests are rejected. This custom-header pattern relies on the API exposing no CORS access to other origins.

Session management follows [OWASP guidance](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html); the custom-header CSRF approach is described in [OWASP's CSRF guidance](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html).

## Local setup

Requirements: Go as specified in `go.mod`, Node.js 24 LTS, npm, Docker Compose for Redis, and a MongoDB connection (local MongoDB or Atlas).

1. Copy `.env.example` to `.env` in the repository root. Set `MONGODB_URI` to your connection string. `.env` is ignored by Git and loaded automatically by Go; exported environment variables take precedence.
2. Set `ADMIN_EMAIL` and `ADMIN_PASSWORD` (12–128 bytes) to create your first admin. Leave both empty if you do not need bootstrap.
3. Start Redis and Go from the repository root:

```sh
docker compose up -d redis
go mod download
go run ./cmd
```

4. In a second terminal:

```sh
cd frontend
npm ci
npm run dev
```

Open **http://localhost:3000**. Sign in with your admin credentials and add a future screening. Sign out and create a regular user to select and confirm a seat.

The root `.env` defaults to `APP_ORIGIN=http://localhost:3000` and `HTTP_ADDR=127.0.0.1:8080`. Open that exact frontend origin; `127.0.0.1:3000` is a different origin. If Go runs elsewhere, set `API_ORIGIN` in `frontend/.env.local` using `frontend/.env.example` as a guide. Set `API_ORIGIN` before building for production: rewrites are captured at build time. Do not put MongoDB credentials in frontend configuration or `NEXT_PUBLIC_*` variables.

For local MongoDB instead of Atlas:

```sh
docker compose --profile local-db up -d mongo
```

For Redis Commander at http://localhost:8081:

```sh
docker compose --profile tools up -d redis-commander
```

Redis and optional local MongoDB bind only to loopback. Both have named data volumes. Redis uses AOF and `noeviction` so memory pressure fails writes rather than silently evicting sessions.

For a production frontend build:

```sh
cd frontend
npm run typecheck
npm run build
npm start
```

## API

All requests with a body use `Content-Type: application/json`. Mutations also require `X-CSRF-Protection: 1`. Authentication comes from cookies.

| Method | Endpoint | Access | Result |
| --- | --- | --- | --- |
| GET | `/health` | Public, direct Go port | Readiness of MongoDB and Redis. |
| POST | `/api/auth/register` | Public | Create USER and set auth cookies. |
| POST | `/api/auth/login` | Public | Verify credentials and set auth cookies. |
| POST | `/api/auth/refresh` | Refresh cookie | Rotate cookies. |
| POST | `/api/auth/logout` | Session cookies | Revoke session; clear cookies. |
| GET | `/api/auth/me` | Signed in | Current user. |
| GET | `/api/movies` | Signed in | Movie screenings. |
| POST | `/api/movies` | ADMIN | Create a screening. |
| DELETE | `/api/movies/{movieID}/bookings/{bookingID}` | Owner | Release a held seat; confirmed bookings cannot be released. |
| GET | `/api/movies/{movieID}/seats` | Signed in | Occupied seats; omit absent/available seats. |
| POST | `/api/movies/{movieID}/bookings` | Signed in | Hold `seat_id` for two minutes. |
| POST | `/api/movies/{movieID}/bookings/{bookingID}/confirm` | Owner | Confirm `seat_id`. |

Registration body: `{"name":"Alex","email":"alex@example.com","password":"a-long-password"}`. Login uses `email` and `password`.

Create a screening with `title`, `synopsis`, optional `poster_url` (a public HTTPS image URL), `genre`, `duration_minutes`, `starts_at` (RFC3339), `rows`, and `seats_per_row`. Rows are 1–12, seats per row 1–16, duration 1–600 minutes, and the start must be in the future. Seat IDs are uppercase, e.g. `A1` or `F10`.

Hold/confirmation/release body: `{"seat_id":"A1"}`. The hold response includes the booking ID and expiry. The server derives ownership from the authenticated user, never from request input. Availability exposes a booking ID only for the requesting user's own seats.

Errors use `{"error":"message"}` with 400 (invalid input), 401 (authentication), 403 (authorization/CSRF), 404 (missing/expired hold), 409 (seat/email conflict), 429 (auth rate limit), or 500 (unexpected failure). Responses include `X-Request-ID` for log correlation.

## Verification

Unit tests do not require running databases:

```sh
go test ./...
go vet ./...
```

Integration tests opt in explicitly and create/drop uniquely named test MongoDB databases. Use disposable local databases, not your production credentials:

```sh
TEST_MONGODB_URI=mongodb://localhost:27017 TEST_REDIS_ADDR=localhost:6380 go test -race ./...
```

Coverage includes concurrent contenders for one seat, expired-hold reclamation, wrong-owner and stale-ID rejection, confirmation retries, access expiry, refresh replay revocation, logout, admin enforcement, strict JSON parsing, CSRF, and booking privacy. Integration tests are skipped when test database variables are absent.

## Boundaries and next steps

This is an MVP teaching architecture. It has no payment, confirmed-booking cancellation, email verification, password recovery, multi-seat checkout, or separate theater/showtime model. Catalog reads are unpaginated and seat availability polls every five seconds. Screenings can supply a poster URL; missing or broken images use decorative photography.

The auth limit is 20 attempts/minute per direct network peer. Behind the Next.js proxy, peers share that budget. Before public deployment, add trusted edge-level client-IP rate limiting, booking abuse limits, and explicit proxy trust rather than accepting arbitrary forwarded headers. Web Locks prevent routine cross-tab refresh races in supporting browsers; unsupported clients must avoid concurrent refreshes.

Use HTTPS with `APP_ORIGIN` set to the public frontend origin, a private Go listener, authenticated/TLS database connections, managed backups, and an external secret provider. Replica-set MongoDB provides majority-write durability; a standalone local database is for development. Keep database clocks/application clocks synchronized for hold deadlines. Redis loss invalidates sessions; confirmed reservations remain in MongoDB. Existing prototype data in Redis is not automatically migrated.
