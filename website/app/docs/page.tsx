import type { Metadata } from "next";
import Link from "next/link";
import { getDocs } from "@/src/lib/docs";

export const metadata: Metadata = {
  title: "Documentation",
  description:
    "Install ascdir and manage App Store metadata, TestFlight, and releases from reviewable files.",
  alternates: { canonical: "/docs/" },
};

export default async function DocsIndex() {
  const docs = await getDocs();
  return (
    <main className="contentPage shell" id="main-content">
      <header className="pageIntro">
        <p className="kicker">Documentation</p>
        <h1>Ship with a plan you can review.</h1>
        <p>
          Start with an existing App Store Connect app, then adopt only the workflows your release
          needs.
        </p>
      </header>
      <div className="docsGrid">
        {docs.map((doc) => (
          <Link className="docCard" href={`/docs/${doc.slug}/`} key={doc.slug}>
            <h2>{doc.title}</h2>
            <p>{doc.description}</p>
            <span>Read documentation →</span>
          </Link>
        ))}
      </div>
    </main>
  );
}
