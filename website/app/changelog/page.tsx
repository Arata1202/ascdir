import type { Metadata } from "next";
import fs from "node:fs/promises";
import path from "node:path";
import { Markdown } from "@/src/components/markdown";

export const metadata: Metadata = {
  title: "Changelog",
  description: "New features, improvements, and fixes released in ascdir.",
  alternates: { canonical: "/changelog/" },
};

function releaseEntries(markdown: string) {
  const unreleased = markdown.match(/^## \[Unreleased\]\s*$/m);
  const firstRelease = markdown.match(/^## \[\d[^\]]*\].*$/m);
  if (!firstRelease?.index) return "";

  if (unreleased?.index !== undefined) {
    const unreleasedBody = markdown
      .slice(unreleased.index + unreleased[0].length, firstRelease.index)
      .trim();
    if (unreleasedBody) return markdown.slice(unreleased.index);
  }

  return markdown.slice(firstRelease.index);
}

export default async function Changelog() {
  const body = await fs.readFile(path.resolve(process.cwd(), "../CHANGELOG.md"), "utf8");
  return (
    <main className="contentPage shell" id="main-content">
      <article className="prose changelogPage">
        <p className="kicker">Release history</p>
        <h1>Changelog</h1>
        <p className="changelogLead">
          Every notable ascdir release, from new capabilities to fixes and security improvements.
        </p>
        <p className="changelogFormat">
          Releases follow <a href="https://semver.org/spec/v2.0.0.html">Semantic Versioning</a> and
          are documented using <a href="https://keepachangelog.com/en/1.1.0/">Keep a Changelog</a>.
        </p>
        <Markdown>{releaseEntries(body)}</Markdown>
      </article>
    </main>
  );
}
