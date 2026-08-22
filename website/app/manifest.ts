import type { MetadataRoute } from "next";

export const dynamic = "force-static";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "ascdir",
    short_name: "ascdir",
    description: "Reviewable App Store release workflows.",
    start_url: "/",
    display: "standalone",
    background_color: "#f7f7f9",
    theme_color: "#f7f7f9",
    icons: [{ src: "/icon.svg", sizes: "any", type: "image/svg+xml" }],
  };
}
