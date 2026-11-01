import type { NextConfig } from "next";

const jobqueueApi = process.env.JOBQUEUE_API_URL ?? "http://127.0.0.1:8080";

const nextConfig: NextConfig = {
  experimental: {
    agentFeedback: true,
  },
  cacheComponents: true,
  partialPrefetching: true,
  async rewrites() {
    return [
      {
        source: "/api/v1/:path*",
        destination: `${jobqueueApi}/api/v1/:path*`,
      },
      {
        source: "/prom/:path*",
        destination: `${process.env.PROMETHEUS_URL ?? "http://127.0.0.1:9090"}/:path*`,
      },
    ];
  },
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
};

export default nextConfig;
