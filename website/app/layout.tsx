import type { Metadata, Viewport } from "next";
import type { ReactNode } from "react";
import { GoogleAnalytics } from "@next/third-parties/google";
import { JsonLd } from "@/src/components/json-ld";
import { SiteFooter } from "@/src/components/site-footer";
import { SiteHeader } from "@/src/components/site-header";
import { site } from "@/src/lib/site";
import "./globals.css";

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  colorScheme: "light",
  themeColor: "#f7f7f9",
};

export const metadata: Metadata = {
  metadataBase: new URL(site.url),
  title: { default: site.title, template: "%s — ascdir" },
  description: site.description,
  alternates: { canonical: "/" },
  icons: { icon: "/icon.svg" },
  openGraph: {
    title: site.title,
    description: site.description,
    siteName: site.name,
    url: "/",
    type: "website",
    locale: "en_US",
    images: [{ url: "/og.png", width: 1280, height: 640, alt: "ascdir release workflow" }],
  },
  twitter: {
    card: "summary_large_image",
    title: site.title,
    description: site.description,
    images: ["/og.png"],
  },
  robots: {
    index: true,
    follow: true,
    googleBot: { index: true, follow: true, "max-image-preview": "large" },
  },
  verification: process.env.NEXT_PUBLIC_GOOGLE_SITE_VERIFICATION
    ? { google: process.env.NEXT_PUBLIC_GOOGLE_SITE_VERIFICATION }
    : undefined,
};

export default function RootLayout({ children }: Readonly<{ children: ReactNode }>) {
  const analyticsId = process.env.NEXT_PUBLIC_GOOGLE_ANALYTICS_ID;
  const websiteJsonLd = {
    "@context": "https://schema.org",
    "@type": "WebSite",
    "@id": `${site.url}#website`,
    name: site.name,
    url: site.url,
    description: site.description,
    inLanguage: "en",
  };

  return (
    <html lang="en" data-scroll-behavior="smooth">
      <body>
        <a className="skipLink" href="#main-content">
          Skip to content
        </a>
        <JsonLd data={websiteJsonLd} />
        <SiteHeader />
        {children}
        <SiteFooter />
        {analyticsId ? <GoogleAnalytics gaId={analyticsId} /> : null}
      </body>
    </html>
  );
}
