import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Page not found",
  robots: { index: false, follow: false },
  alternates: { canonical: null },
};

export default function NotFound() {
  return (
    <main className="emptyPage shell" id="main-content">
      <p className="kicker">404</p>
      <h1>This page does not exist.</h1>
      <p>The address may have changed, or the page may have been removed.</p>
      <Link className="button buttonPrimary" href="/">
        Return home
      </Link>
    </main>
  );
}
