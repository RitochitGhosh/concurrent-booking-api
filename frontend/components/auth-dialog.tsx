"use client";
import { useState, type FormEvent } from "react";
import { api, post } from "@/lib/api";
import type { User } from "@/lib/types";
import { Modal } from "./modal";

export function AuthDialog({
  onClose,
  onUser,
}: {
  onClose: () => void;
  onUser: (user: User) => void;
}) {
  const [register, setRegister] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const fields = new FormData(event.currentTarget);
    setBusy(true);
    setError("");
    try {
      const input = {
        email: fields.get("email"),
        password: fields.get("password"),
        ...(register ? { name: fields.get("name") } : {}),
      };
      const user = await api<User>(
        `/api/auth/${register ? "register" : "login"}`,
        post(input),
        false,
      );
      onUser(user);
      onClose();
    } catch (error) {
      setError(error instanceof Error ? error.message : "Unable to sign in.");
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal className="auth-dialog" titleID="auth-title" onClose={onClose}>
      <div className="auth-art">
        <span className="eyebrow">WELCOME TO FRAME</span>
        <h2>
          Good stories.
          <br />
          Better company.
        </h2>
        <p>Your seat is waiting.</p>
      </div>
      <div className="auth-content">
        <div className="auth-tabs">
          <button
            disabled={busy}
            className={!register ? "active" : ""}
            onClick={() => {
              setRegister(false);
              setError("");
            }}
          >
            Sign in
          </button>
          <button
            disabled={busy}
            className={register ? "active" : ""}
            onClick={() => {
              setRegister(true);
              setError("");
            }}
          >
            Create account
          </button>
        </div>
        <h2 id="auth-title">
          {register ? "Your next story awaits." : "Welcome back."}
        </h2>
        <p className="muted">
          {register
            ? "Join us for a little big-screen escape."
            : "Let’s find you something worth watching."}
        </p>
        <form onSubmit={submit}>
          {register && (
            <label>
              Your name
              <input name="name" autoComplete="name" maxLength={80} required />
            </label>
          )}
          <label>
            Email address
            <input
              name="email"
              type="email"
              autoComplete="email"
              maxLength={254}
              required
              placeholder="you@example.com"
            />
          </label>
          <label>
            Password
            <input
              name="password"
              type="password"
              autoComplete={register ? "new-password" : "current-password"}
              minLength={register ? 12 : 1}
              maxLength={128}
              required
              placeholder="Your password"
            />
          </label>
          {register && (
            <p className="muted fine">Use at least 12 characters.</p>
          )}
          <p className="form-error" role="alert">
            {error}
          </p>
          <button className="button lime full" type="submit" disabled={busy}>
            {busy ? "One moment…" : register ? "Create account ↗" : "Sign in ↗"}
          </button>
        </form>
        <p className="fine muted">A seat, a story, a little escape.</p>
      </div>
    </Modal>
  );
}
