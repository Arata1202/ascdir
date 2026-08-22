export const site = {
  name: "ascdir",
  title: "ascdir — Reviewable App Store releases",
  description:
    "Manage App Store metadata and product-page assets as files, then review TestFlight and release operations with safe dry runs.",
  url: process.env.NEXT_PUBLIC_SITE_URL ?? "https://ascdir.pages.dev",
  github: "https://github.com/Arata1202/ascdir",
} as const;

export function absoluteUrl(path = "/") {
  return new URL(path, site.url).toString();
}
