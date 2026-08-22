"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { Brand } from "./brand";
import { site } from "@/src/lib/site";

function MobileMenu() {
  const [menuOpen, setMenuOpen] = useState(false);
  const closeMenu = () => setMenuOpen(false);

  return (
    <details
      className="mobileMenu"
      open={menuOpen}
      onToggle={(event) => setMenuOpen(event.currentTarget.open)}
      onKeyDown={(event) => {
        if (event.key === "Escape") closeMenu();
      }}
    >
      <summary
        aria-controls="mobile-navigation"
        aria-expanded={menuOpen}
        aria-label={menuOpen ? "Close navigation" : "Open navigation"}
      >
        <span className="menuIcon" aria-hidden="true">
          <span />
          <span />
        </span>
      </summary>
      <div id="mobile-navigation">
        <Link href="/docs/" onClick={closeMenu}>
          Docs
        </Link>
        <Link href="/comparison/fastlane/" onClick={closeMenu}>
          Comparison
        </Link>
        <Link href="/changelog/" onClick={closeMenu}>
          Changelog
        </Link>
        <a className="navGithub" href={site.github} onClick={closeMenu}>
          GitHub ↗
        </a>
      </div>
    </details>
  );
}

export function SiteHeader() {
  const pathname = usePathname();

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
        <MobileMenu key={pathname} />
      </nav>
    </header>
  );
}
