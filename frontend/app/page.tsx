"use client";
import { useCallback, useEffect, useState } from "react";
import { api, APIError } from "@/lib/api";
import { displayTime, type Movie, type User } from "@/lib/types";
import { AuthDialog } from "@/components/auth-dialog";
import { AdminDialog } from "@/components/admin-dialog";
import { SeatPicker } from "@/components/seat-picker";

const photos = [
  "photo-1470770841072-f978cf4d019e",
  "photo-1440404653325-ab127d49abc1",
  "photo-1519608487953-e999c86e7455",
  "photo-1518837695005-2083093ee35b",
  "photo-1475924156734-496f6cac6ec1",
];
const genres = ["all", "Drama", "Action", "Sci-Fi", "Other"];

export default function Home() {
  const [user, setUser] = useState<User | null>(null);
  const [movies, setMovies] = useState<Movie[]>([]);
  const [filter, setFilter] = useState("all");
  const [dialog, setDialog] = useState<"auth" | "admin" | null>(null);
  const [selectedMovie, setSelectedMovie] = useState<Movie | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [signingOut, setSigningOut] = useState(false);
  const loadMovies = useCallback(async () => {
    setError("");
    setLoading(true);
    try {
      setMovies(await api<Movie[]>("/api/movies"));
    } catch (error) {
      setError(
        error instanceof Error ? error.message : "Unable to load screenings.",
      );
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => {
    const signedOut = () => {
      setUser(null);
      setMovies([]);
      setSelectedMovie(null);
      setDialog(null);
    };
    window.addEventListener("frame:signed-out", signedOut);
    let active = true;
    void (async () => {
      try {
        const current = await api<User>("/api/auth/me");
        if (active) {
          setUser(current);
          await loadMovies();
        }
      } catch (error) {
        if (active && !(error instanceof APIError && error.status === 401))
          setError(
            error instanceof Error ? error.message : "Unable to reach the API.",
          );
      } finally {
        if (active) setLoading(false);
      }
    })();
    return () => {
      active = false;
      window.removeEventListener("frame:signed-out", signedOut);
    };
  }, [loadMovies]);
  async function signOut() {
    setSigningOut(true);
    try {
      await api<void>("/api/auth/logout", { method: "POST" }, false);
      setUser(null);
      setMovies([]);
      setSelectedMovie(null);
      setDialog(null);
      setError("");
    } catch (error) {
      setError(error instanceof Error ? error.message : "Unable to sign out.");
    } finally {
      setSigningOut(false);
    }
  }
  const upcoming = movies.filter(
    (m) => new Date(m.starts_at).getTime() > Date.now(),
  );
  const visible = upcoming.filter(
    (m) =>
      filter === "all" ||
      (filter === "Other"
        ? !["Drama", "Action", "Sci-Fi"].includes(m.genre)
        : m.genre === filter),
  );
  return (
    <>
      <header className="header">
        <a className="logo" href="/" aria-label="Frame home">
          <span className="logo-mark">F</span>FRAME
          <span className="logo-dot">®</span>
        </a>
        <nav aria-label="Main navigation">
          <a href="#showings">Now showing</a>
          <span className="nav-note">A better seat. A bigger story.</span>
        </nav>
        <div className="account">
          {user && <span id="user-name">Hi, {user.name.split(" ")[0]}</span>}
          {user?.role === "ADMIN" && (
            <button
              className="button small secondary"
              onClick={() => setDialog("admin")}
            >
              Add screening
            </button>
          )}
          <button
            className="button small"
            disabled={signingOut}
            onClick={() => (user ? void signOut() : setDialog("auth"))}
          >
            {user ? "Sign out ↗" : "Sign in ↗"}
          </button>
        </div>
      </header>
      <main>
        <section className="hero" aria-labelledby="hero-title">
          <img
            className="hero-image"
            src="https://images.unsplash.com/photo-1489599849927-2ee91cede3ba?auto=format&fit=crop&w=2000&q=85"
            alt="Rows of plush red seats in a softly lit cinema"
            fetchPriority="high"
          />
          <div className="hero-shade" />
          <div className="hero-top">
            <span>
              <i className="live-dot" />
              THE BIG SCREEN IS CALLING
            </span>
            <span>YOUR NEXT GREAT EVENING</span>
          </div>
          <div className="hero-copy">
            <p className="eyebrow">LESS SCROLLING. MORE CINEMA.</p>
            <h1 id="hero-title">
              Some stories deserve
              <br />
              the <em>big screen.</em>
            </h1>
            <p>
              Find your film. Pick your seat.
              <br />
              Let the rest of the world wait.
            </p>
            <a className="button lime" href="#showings">
              Explore screenings <span aria-hidden="true">↗</span>
            </a>
          </div>
          <div className="hero-bottom">
            <span>01 / THE CINEMA EXPERIENCE</span>
            <span>Lights down. Phones away. You’re here.</span>
          </div>
        </section>
        <section
          id="showings"
          className="catalog"
          aria-labelledby="catalog-title"
        >
          <div className="section-heading">
            <div>
              <p className="eyebrow">MAKE A NIGHT OF IT</p>
              <h2 id="catalog-title">
                On the big screen<span className="accent">.</span>
              </h2>
            </div>
            <span className="section-caption">
              <i className="live-dot" />
              Real-time seat availability
            </span>
          </div>
          <div className="catalog-toolbar">
            <div
              className="filters"
              role="group"
              aria-label="Filter screenings"
            >
              {genres.map((g) => (
                <button
                  key={g}
                  className={`filter${filter === g ? " active" : ""}`}
                  aria-pressed={filter === g}
                  onClick={() => setFilter(g)}
                >
                  {g === "all"
                    ? "All films"
                    : g === "Other"
                      ? "More stories"
                      : g}
                </button>
              ))}
            </div>
            <span id="screening-count">
              {user
                ? `${upcoming.length} upcoming screening${upcoming.length === 1 ? "" : "s"}`
                : "Your next story awaits"}
            </span>
          </div>
          {error && (
            <div className="catalog-error" role="alert">
              <p>{error}</p>
              <button
                className="button small secondary"
                onClick={() =>
                  user ? void loadMovies() : window.location.reload()
                }
              >
                Try again
              </button>
            </div>
          )}
          {loading ? (
            <div className="empty-state" role="status">
              <span className="empty-symbol">◌</span>
              <h3>Setting the scene…</h3>
            </div>
          ) : !user ? (
            <div className="empty-state">
              <span className="empty-symbol">↗</span>
              <h3>Your evening starts here.</h3>
              <p>Sign in to discover screenings and find your favorite seat.</p>
              <button className="button lime" onClick={() => setDialog("auth")}>
                Find my next film
              </button>
            </div>
          ) : !visible.length ? (
            <div className="empty-state">
              <span className="empty-symbol">◌</span>
              <h3>
                {upcoming.length
                  ? "A different kind of story?"
                  : "The next act is coming."}
              </h3>
              <p>
                {upcoming.length
                  ? "Try another genre to see more screenings."
                  : user.role === "ADMIN"
                    ? "Add your first screening using the button above."
                    : "No upcoming screenings yet. Check back soon."}
              </p>
            </div>
          ) : (
            <div className="movie-grid">
              {visible.map((m, i) => (
                <article key={m.id} className="movie-card">
                  <div className="movie-art">
                    <img
                      src={
                        m.poster_url ||
                        `https://images.unsplash.com/${photos[i % photos.length]}?auto=format&fit=crop&w=750&q=80`
                      }
                      onError={(event) => {
                        const fallback = `https://images.unsplash.com/${photos[i % photos.length]}?auto=format&fit=crop&w=750&q=80`;
                        if (event.currentTarget.src !== fallback)
                          event.currentTarget.src = fallback;
                      }}
                      alt={m.poster_url ? `Poster for ${m.title}` : ""}
                      loading="lazy"
                    />
                    <span className="movie-tag">
                      {m.genre} · {m.duration_minutes} MIN
                    </span>
                    <span className="art-title">{m.title}</span>
                  </div>
                  <h3>{m.title}</h3>
                  <div className="movie-details">
                    {m.genre} &nbsp; · &nbsp; {m.duration_minutes} minutes
                  </div>
                  <p className="movie-synopsis">
                    {m.synopsis ||
                      "A story best experienced on the big screen."}
                  </p>
                  <div className="movie-bottom">
                    <span>{displayTime(m.starts_at)}</span>
                    <button onClick={() => setSelectedMovie(m)}>
                      Pick your seat ↗
                    </button>
                  </div>
                </article>
              ))}
            </div>
          )}
        </section>
        <section className="experience">
          <div>
            <p className="eyebrow">THE RITUAL, REIMAGINED</p>
            <h2>
              Just you.
              <br />
              And the story.
            </h2>
          </div>
          <div className="experience-item">
            <span>01</span>
            <h3>Choose your moment.</h3>
            <p>
              Browse upcoming screenings and find a story that speaks to you.
            </p>
          </div>
          <div className="experience-item">
            <span>02</span>
            <h3>Make it your seat.</h3>
            <p>
              Pick a spot on the live seating map. We’ll hold it for two
              minutes.
            </p>
          </div>
          <div className="experience-item">
            <span>03</span>
            <h3>You’re on the list.</h3>
            <p>
              Confirm your reservation. No payment step. Just a night at the
              movies.
            </p>
          </div>
        </section>
      </main>
      <footer>
        <a className="logo" href="/">
          FRAME<span className="logo-dot">®</span>
        </a>
        <p>For the love of the big screen.</p>
        <span>A LITTLE ESCAPE GOES A LONG WAY.</span>
      </footer>
      {dialog === "auth" && (
        <AuthDialog
          onClose={() => setDialog(null)}
          onUser={(u) => {
            setUser(u);
            void loadMovies();
          }}
        />
      )}{" "}
      {dialog === "admin" && user?.role === "ADMIN" && (
        <AdminDialog
          onClose={() => setDialog(null)}
          onCreated={() => {
            setFilter("all");
            void loadMovies();
          }}
        />
      )}{" "}
      {selectedMovie && user && (
        <SeatPicker
          key={selectedMovie.id}
          movie={selectedMovie}
          onClose={() => setSelectedMovie(null)}
        />
      )}
    </>
  );
}
