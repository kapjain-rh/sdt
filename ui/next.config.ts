import type { NextConfig } from "next";
import path from "path";

const isStaticExport = process.env.STATIC_EXPORT === "1";

const nextConfig: NextConfig = {
  turbopack: {
    root: path.resolve(__dirname),
  },
  ...(isStaticExport
    ? {
        output: "export",
        images: { unoptimized: true },
      }
    : {
        async rewrites() {
          return [
            {
              source: "/api/:path*",
              destination: `http://localhost:${process.env.SDT_API_PORT || "8090"}/api/:path*`,
            },
          ];
        },
      }),
};

export default nextConfig;
