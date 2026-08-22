import Link from "next/link";
import { Brand } from "./brand";
import { site } from "@/src/lib/site";

export function SiteHeader() {
  return (
    <header className="siteHeader">
      <nav className="nav shell" aria-label="Primary navigation">
        <Brand />
        <div className="navLinks">
          <Link href="/docs/">Docs</Link>
          <Link href="/comparison/fastlane/">Comparison</Link>
          <Link href="/changelog/">Changelog</Link>
          <a className="navGithub" href={site.github}>
            GitHub ↗
          </a>
        </div>
        <details className="mobileMenu">
          <summary aria-label="Open navigation">Menu</summary>
          <div>
            <Link href="/docs/">Docs</Link>
            <Link href="/comparison/fastlane/">Comparison</Link>
            <Link href="/changelog/">Changelog</Link>
            <a className="navGithub" href={site.github}>
              GitHub ↗
            </a>
          </div>
        </details>
      </nav>
    </header>
  );
}
