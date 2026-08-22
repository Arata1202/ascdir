import type { Metadata } from "next";

export const site = {
  name: "ascdir",
  title: "ascdir — Reviewable App Store releases",
  description:
    "Manage App Store metadata and product-page assets as files, then review TestFlight and release operations with safe dry runs.",
  url: process.env.NEXT_PUBLIC_SITE_URL ?? "https://ascdir.pages.dev",
  github: "https://github.com/Arata1202/ascdir",
} as const;

export function isCloudflarePreview(
  env: Readonly<Record<string, string | undefined>> = process.env,
) {
  return env.CF_PAGES === "1" && env.CF_PAGES_BRANCH !== "main";
}

export const isPreviewDeployment = isCloudflarePreview();

export function absoluteUrl(path = "/") {
  return new URL(path, site.url).toString();
}

export function pageMetadata({
  title,
  description,
  path,
  type = "website",
}: {
  title: string;
  description: string;
  path: string;
  type?: "website" | "article";
}): Metadata {
  return {
    title,
    description,
    alternates: { canonical: path },
    openGraph: {
      title,
      description,
      url: path,
      type,
      siteName: site.name,
      locale: "en_US",
      images: [{ url: "/og.png", width: 1280, height: 640, alt: "ascdir release workflow" }],
    },
    twitter: {
      card: "summary_large_image",
      title,
      description,
      images: ["/og.png"],
    },
  };
}
