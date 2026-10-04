import type { Metadata } from "next";
import "./globals.css";
export const metadata: Metadata = {
  title: "FRAME — An evening worth remembering",
  description: "Find your film, choose your seat, and make a night of it.",
};
export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
