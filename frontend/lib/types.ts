export type User = {
  id: string;
  name: string;
  email: string;
  role: "USER" | "ADMIN";
};
export type Movie = {
  id: string;
  title: string;
  synopsis: string;
  poster_url?: string;
  genre: string;
  duration_minutes: number;
  starts_at: string;
  rows: number;
  seats_per_row: number;
};
export type Seat = {
  seat_id: string;
  status: "held" | "confirmed";
  mine: boolean;
  booking_id?: string;
  expires_at?: string;
};
export type Booking = {
  id: string;
  movie_id: string;
  seat_id: string;
  status: "held" | "confirmed";
  expires_at: string;
};
export const displayTime = (value: string) =>
  new Date(value).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
