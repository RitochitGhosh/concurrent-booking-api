"use client";
import {
  Fragment,
  useCallback,
  useEffect,
  useRef,
  useState,
  type CSSProperties,
} from "react";
import { api, post } from "@/lib/api";
import { displayTime, type Movie, type Seat, type Booking } from "@/lib/types";
import { Modal } from "./modal";

export function SeatPicker({
  movie,
  onClose,
}: {
  movie: Movie;
  onClose: () => void;
}) {
  const [seats, setSeats] = useState<Seat[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState("");
  const [now, setNow] = useState(Date.now());
  const version = useRef(0);
  const working = useRef(false);
  const mounted = useRef(true);
  const refresh = useCallback(async () => {
    if (working.current) return;
    const request = ++version.current;
    try {
      const result = await api<Seat[]>(`/api/movies/${movie.id}/seats`);
      if (mounted.current && request === version.current) {
        setSeats(result);
        setLoaded(true);
      }
    } catch (error) {
      if (mounted.current && request === version.current)
        setError(
          error instanceof Error ? error.message : "Unable to load seats.",
        );
    }
  }, [movie.id]);
  useEffect(() => {
    mounted.current = true;
    void refresh();
    const poll = setInterval(refresh, 5000);
    const clock = setInterval(() => setNow(Date.now()), 1000);
    return () => {
      mounted.current = false;
      version.current++;
      clearInterval(poll);
      clearInterval(clock);
    };
  }, [refresh]);
  const seat = seats.find((s) => s.seat_id === selected);
  const held = seat?.mine && seat.status === "held";
  const confirmed = seat?.mine && seat.status === "confirmed";
  const conflict = seat && !seat.mine;
  const remaining = held
    ? Math.max(
        0,
        Math.ceil((new Date(seat.expires_at!).getTime() - now) / 1000),
      )
    : 0;
  async function reserve() {
    if (!selected || working.current) return;
    working.current = true;
    version.current++;
    setBusy(true);
    setError("");
    try {
      const path = held
        ? `/api/movies/${movie.id}/bookings/${seat!.booking_id}/confirm`
        : `/api/movies/${movie.id}/bookings`;
      const b = await api<Booking>(path, post({ seat_id: selected }));
      if (mounted.current)
        setSeats((current) => [
          ...current.filter((s) => s.seat_id !== selected),
          {
            seat_id: b.seat_id,
            mine: true,
            status: b.status,
            booking_id: b.id,
            expires_at: b.expires_at,
          },
        ]);
    } catch (error) {
      if (mounted.current)
        setError(
          error instanceof Error ? error.message : "Unable to reserve seat.",
        );
    } finally {
      working.current = false;
      if (mounted.current) {
        setBusy(false);
        void refresh();
      }
    }
  }
  async function releaseHold() {
    if (!selected || !held || working.current) return;
    working.current = true;
    version.current++;
    setBusy(true);
    setError("");
    try {
      await api<void>(`/api/movies/${movie.id}/bookings/${seat!.booking_id}`, {
        ...post({ seat_id: selected }),
        method: "DELETE",
      });
      if (mounted.current) {
        setSeats((current) => current.filter((s) => s.seat_id !== selected));
        setSelected(null);
      }
    } catch (error) {
      if (mounted.current)
        setError(
          error instanceof Error ? error.message : "Unable to release seat.",
        );
    } finally {
      working.current = false;
      if (mounted.current) {
        setBusy(false);
        void refresh();
      }
    }
  }
  return (
    <Modal className="booking-dialog" titleID="booking-title" onClose={onClose}>
      <div className="booking-heading">
        <p className="eyebrow">THE BEST VIEW IS YOURS</p>
        <h2 id="booking-title">{movie.title}</h2>
        <p className="muted">
          {movie.genre} · {movie.duration_minutes} minutes ·{" "}
          {displayTime(movie.starts_at)}
        </p>
      </div>
      <div className="booking-body">
        <section className="auditorium" aria-label="Seat selection">
          <div className="screen">
            <span>THE BIG SCREEN</span>
          </div>
          <div
            className="seat-grid"
            style={{ "--columns": movie.seats_per_row } as CSSProperties}
          >
            {Array.from({ length: movie.rows }, (_, row) => {
              const letter = String.fromCharCode(65 + row);
              return (
                <Fragment key={letter}>
                  <span className="row-label">{letter}</span>
                  {Array.from({ length: movie.seats_per_row }, (_, col) => {
                    const id = letter + (col + 1);
                    const occupied = seats.find((s) => s.seat_id === id);
                    const unavailable = occupied && !occupied.mine;
                    return (
                      <button
                        key={id}
                        className={`seat${unavailable ? " taken" : ""}${occupied?.mine ? " mine" : ""}${selected === id ? " selected" : ""}`}
                        disabled={!!unavailable || busy || !loaded}
                        aria-label={`Seat ${id}, ${occupied ? (occupied.mine ? `your ${occupied.status} seat` : "unavailable") : "available"}`}
                        aria-pressed={selected === id}
                        onClick={() => {
                          setSelected(id);
                          setError("");
                        }}
                      >
                        {col + 1}
                      </button>
                    );
                  })}
                </Fragment>
              );
            })}
          </div>
          <div className="legend">
            <span>
              <i className="seat-icon" />
              Available
            </span>
            <span>
              <i className="seat-icon selected" />
              Selected
            </span>
            <span>
              <i className="seat-icon taken" />
              Unavailable
            </span>
            <span>
              <i className="seat-icon mine" />
              Yours
            </span>
          </div>
          <p className="fine muted">
            {loaded
              ? "Seats update every 5 seconds. Your hold secures your seat."
              : "Loading live seat availability…"}
          </p>
        </section>
        <aside className="reservation">
          <p className="eyebrow">YOUR EVENING</p>
          <h3>
            {confirmed
              ? "The seat is yours."
              : held
                ? `Seat ${selected} is on hold.`
                : selected
                  ? `A great view from ${selected}.`
                  : "Find your favorite spot."}
          </h3>
          <p className="muted">
            {confirmed
              ? "Your reservation is ready. Enjoy the show."
              : held
                ? "One last step: confirm your seat before the timer runs out."
                : conflict
                  ? "Someone else picked this seat. Choose another."
                  : selected
                    ? "Hold this seat for two minutes, then confirm your reservation."
                    : "Choose an available seat to get started."}
          </p>
          <div className="reservation-line">
            <span>Screening</span>
            <strong>{displayTime(movie.starts_at)}</strong>
          </div>
          <div className="reservation-line">
            <span>Seat</span>
            <strong>{selected ?? "—"}</strong>
          </div>
          {held && (
            <div className="hold-timer" role="timer">
              {remaining
                ? `Your hold expires in ${Math.floor(remaining / 60)}:${String(remaining % 60).padStart(2, "0")}`
                : "Hold expired. Refreshing availability…"}
            </div>
          )}
          <p className="form-error" role="alert">
            {error}
          </p>
          <button
            className="button lime full"
            onClick={reserve}
            disabled={
              busy ||
              !selected ||
              !loaded ||
              !!confirmed ||
              !!conflict ||
              (!!held && !remaining)
            }
          >
            {busy
              ? "One moment…"
              : confirmed
                ? "Reservation confirmed ✓"
                : held
                  ? "Confirm my booking ↗"
                  : selected
                    ? "Hold this seat ↗"
                    : "Select a seat"}
          </button>
          {held && (
            <button
              className="button secondary full release-hold"
              onClick={releaseHold}
              disabled={busy}
            >
              Release held seat
            </button>
          )}
          <p className="fine muted">
            No payment required. Confirm your seat and you’re all set.
          </p>
          {confirmed && (
            <div className="confirmation">
              <span>✓</span>
              <h3>See you at the movies.</h3>
              <p>Your reservation is confirmed.</p>
              <code>{seat!.booking_id}</code>
            </div>
          )}
        </aside>
      </div>
    </Modal>
  );
}
