import type { Metadata } from "next";
import fs from "node:fs/promises";
import path from "node:path";
import { Markdown } from "@/src/components/markdown";

export const metadata: Metadata = {
  title: "Changelog",
  description: "New features, improvements, and fixes released in ascdir.",
  alternates: { canonical: "/changelog/" },
};

export default async function Changelog() {
  const body = await fs.readFile(path.resolve(process.cwd(), "../CHANGELOG.md"), "utf8");
  return (
    <main className="docLayout shell" id="main-content">
      <aside className="docAside">
        <span>Release history</span>
      </aside>
      <article className="prose">
        <p className="kicker">Changelog</p>
        <Markdown>{body.replace(/^#\s+.+\r?\n+/, "")}</Markdown>
      </article>
    </main>
  );
}
