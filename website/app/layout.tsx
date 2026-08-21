import type { Metadata } from "next";
import type { ReactNode } from "react";
import "./globals.css";

export const metadata: Metadata = {
  title: "ascdir — Reviewable App Store releases",
  description:
    "Manage App Store metadata, TestFlight distribution, and releases as reviewable files with safe dry-run workflows.",
  openGraph: {
    title: "ascdir — Reviewable App Store releases",
    description:
      "Manage App Store metadata, TestFlight distribution, and releases as reviewable files.",
    images: [
      {
        url: "https://raw.githubusercontent.com/Arata1202/ascdir/main/.github/social-preview.png",
        width: 1280,
        height: 640,
      },
    ],
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "ascdir — Reviewable App Store releases",
    description:
      "Manage App Store metadata, TestFlight distribution, and releases as reviewable files.",
    images: [
      "https://raw.githubusercontent.com/Arata1202/ascdir/main/.github/social-preview.png",
    ],
  },
};

export default function RootLayout({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
