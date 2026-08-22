import Link from "next/link";
import { Brand } from "./brand";
import { site } from "@/src/lib/site";

export function SiteFooter() {
  return (
    <footer className="footer shell">
      <Brand />
      <p>Reviewable App Store workflows, released under the MIT License.</p>
      <div>
        <Link href="/docs/">Docs</Link>
        <a href={`${site.github}/releases`}>Releases</a>
        <a href={`${site.github}/blob/main/LICENSE`}>License</a>
      </div>
    </footer>
  );
}
