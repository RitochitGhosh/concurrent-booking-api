import type { NextConfig } from "next";

// The database connection string belongs only to Go's root .env.
// This internal backend address is never exposed as NEXT_PUBLIC_*.
const apiOrigin = process.env.API_ORIGIN ?? "http://127.0.0.1:8080";
const config: NextConfig = {
  poweredByHeader: false,
  agentRules: false,
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${apiOrigin}/api/:path*` }];
  },
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "X-Frame-Options", value: "DENY" },
          { key: "Referrer-Policy", value: "same-origin" },
        ],
      },
    ];
  },
};
export default config;
