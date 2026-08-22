import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { JsonLd } from "@/src/components/json-ld";
import { Markdown } from "@/src/components/markdown";
import { DocToc } from "@/src/components/doc-toc";
import { getDoc, getDocs, getDocSlugs } from "@/src/lib/docs";
import { absoluteUrl } from "@/src/lib/site";

type Props = { params: Promise<{ slug: string }> };

export async function generateStaticParams() {
  return (await getDocSlugs()).map((slug) => ({ slug }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  const doc = await getDoc(slug);
  if (!doc) return {};
  return {
    title: doc.title,
    description: doc.description,
    alternates: { canonical: `/docs/${slug}/` },
    openGraph: {
      title: doc.title,
      description: doc.description,
      url: `/docs/${slug}/`,
      type: "article",
    },
  };
}

export default async function DocPage({ params }: Props) {
  const { slug } = await params;
  const doc = await getDoc(slug);
  if (!doc) notFound();
  const docs = await getDocs();
  const currentIndex = docs.findIndex((item) => item.slug === slug);
  const previousDoc = currentIndex > 0 ? docs[currentIndex - 1] : null;
  const nextDoc = currentIndex < docs.length - 1 ? docs[currentIndex + 1] : null;
  const url = absoluteUrl(`/docs/${slug}/`);
  const breadcrumb = {
    "@context": "https://schema.org",
    "@type": "BreadcrumbList",
    itemListElement: [
      { "@type": "ListItem", position: 1, name: "Home", item: absoluteUrl("/") },
      { "@type": "ListItem", position: 2, name: "Documentation", item: absoluteUrl("/docs/") },
      { "@type": "ListItem", position: 3, name: doc.title, item: url },
    ],
  };
  return (
    <main className="docLayout shell" id="main-content">
      <aside className="docAside" aria-label="Documentation navigation">
        <Link href="/docs/">← All documentation</Link>
        {doc.toc.length > 0 ? <DocToc items={doc.toc} /> : null}
      </aside>
      <article className="prose">
        <JsonLd data={breadcrumb} />
        <p className="kicker">Documentation</p>
        <h1>{doc.title}</h1>
        <Markdown>{doc.body}</Markdown>
        <nav className="docPager" aria-label="Documentation pages">
          {previousDoc ? (
            <Link href={`/docs/${previousDoc.slug}/`}>
              <span>Previous</span>
              {previousDoc.title}
            </Link>
          ) : (
            <span />
          )}
          {nextDoc ? (
            <Link href={`/docs/${nextDoc.slug}/`}>
              <span>Next</span>
              {nextDoc.title}
            </Link>
          ) : null}
        </nav>
      </article>
    </main>
  );
}
