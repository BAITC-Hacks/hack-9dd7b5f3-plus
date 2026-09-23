import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Standalone output is what the Dockerfile copies (.next/standalone + static).
  output: "standalone",
};

export default nextConfig;
