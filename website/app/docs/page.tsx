import type { Metadata } from "next";
import Link from "next/link";
import { docCategories, getDocs } from "@/src/lib/docs";

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
      <div className="docsGroups">
        {docCategories.map((category) => {
          const categoryDocs = docs.filter((doc) => doc.category === category);
          if (categoryDocs.length === 0) return null;
          return (
            <section className="docsGroup" aria-labelledby={`category-${category}`} key={category}>
              <h2 id={`category-${category}`}>{category}</h2>
              <div className="docsGrid">
                {categoryDocs.map((doc) => (
                  <Link className="docCard" href={`/docs/${doc.slug}/`} key={doc.slug}>
                    <h3>{doc.title}</h3>
                    <p>{doc.description}</p>
                    <span>Read documentation →</span>
                  </Link>
                ))}
              </div>
            </section>
          );
        })}
      </div>
    </main>
  );
}
