import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { JsonLd } from "@/src/components/json-ld";
import { Markdown } from "@/src/components/markdown";
import { getDoc, getDocSlugs } from "@/src/lib/docs";
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
      </aside>
      <article className="prose">
        <JsonLd data={breadcrumb} />
        <p className="kicker">Documentation</p>
        <h1>{doc.title}</h1>
        <Markdown>{doc.body}</Markdown>
      </article>
    </main>
  );
}
