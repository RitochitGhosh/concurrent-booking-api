"use client";
import { useState, type FormEvent } from "react";
import { api, post } from "@/lib/api";
import type { Movie } from "@/lib/types";
import { Modal } from "./modal";
import { ScreeningTimePicker } from "./screening-time-picker";

export function AdminDialog({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: () => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const fields = Object.fromEntries(new FormData(event.currentTarget));
    setBusy(true);
    setError("");
    try {
      const startsAt = new Date(String(fields.starts_at));
      if (
        !Number.isFinite(startsAt.getTime()) ||
        startsAt.getTime() <= Date.now()
      ) {
        setError("Choose a future screening date and time.");
        return;
      }
      const input = {
        ...fields,
        starts_at: startsAt.toISOString(),
        duration_minutes: Number(fields.duration_minutes),
        rows: Number(fields.rows),
        seats_per_row: Number(fields.seats_per_row),
      };
      await api<Movie>("/api/movies", post(input));
      onCreated();
      onClose();
    } catch (error) {
      setError(
        error instanceof Error ? error.message : "Unable to publish screening.",
      );
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal className="admin-dialog" titleID="admin-title" onClose={onClose}>
      <p className="eyebrow">THE PROJECTION ROOM · ADMIN</p>
      <h2 id="admin-title">A new story starts here.</h2>
      <p className="muted">Add one movie screening and its seating layout.</p>
      <form onSubmit={submit}>
        <label>
          Movie title
          <input
            name="title"
            maxLength={120}
            required
            placeholder="The name on the marquee"
          />
        </label>
        <label>
          Poster image URL (optional)
          <input
            name="poster_url"
            type="url"
            maxLength={2048}
            pattern="https://.*"
            placeholder="https://example.com/poster.jpg"
          />
          <span className="fine muted">
            Use a publicly accessible HTTPS image URL.
          </span>
        </label>
        <label>
          Synopsis
          <textarea
            name="synopsis"
            maxLength={2000}
            rows={3}
            placeholder="Give your audience a taste of the story."
          />
        </label>
        <div className="form-row">
          <label>
            Genre
            <select name="genre">
              {[
                "Drama",
                "Action",
                "Sci-Fi",
                "Comedy",
                "Thriller",
                "Documentary",
                "Animation",
              ].map((g) => (
                <option key={g}>{g}</option>
              ))}
            </select>
          </label>
          <label>
            Duration (minutes)
            <input
              name="duration_minutes"
              type="number"
              min={1}
              max={600}
              defaultValue={120}
              required
            />
          </label>
        </div>
        <ScreeningTimePicker disabled={busy} />
        <div className="form-row">
          <label>
            Rows (1–12)
            <input
              name="rows"
              type="number"
              min={1}
              max={12}
              defaultValue={6}
              required
            />
          </label>
          <label>
            Seats per row (1–16)
            <input
              name="seats_per_row"
              type="number"
              min={1}
              max={16}
              defaultValue={10}
              required
            />
          </label>
        </div>
        <p className="form-error" role="alert">
          {error}
        </p>
        <button className="button lime full" type="submit" disabled={busy}>
          {busy ? "Publishing…" : "Publish screening ↗"}
        </button>
      </form>
    </Modal>
  );
}
