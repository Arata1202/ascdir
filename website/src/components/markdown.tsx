import Link from "next/link";
import type { ReactNode } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { CodeBlock } from "./code-block";
import { slugifyHeading } from "@/src/lib/docs";

function textFromNode(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(textFromNode).join("");
  if (node && typeof node === "object" && "props" in node) {
    return textFromNode((node as { props: { children?: ReactNode } }).props.children);
  }
  return "";
}

function normalizeHref(href?: string) {
  if (!href) return "#";
  const docLink = href.match(/^(?:\.\/)?([a-z0-9-]+)\.md(#[a-z0-9-]+)?$/i);
  if (docLink) return `/docs/${docLink[1]}/${docLink[2] ?? ""}`;
  return href;
}

export function Markdown({ children }: { children: string }) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      components={{
        pre: ({ children: code }) => <CodeBlock>{code}</CodeBlock>,
        h2: ({ children: heading }) => (
          <h2 id={slugifyHeading(textFromNode(heading))}>{heading}</h2>
        ),
        a: ({ href, children: label }) => {
          const normalized = normalizeHref(href);
          return normalized.startsWith("/") ? (
            <Link href={normalized}>{label}</Link>
          ) : (
            <a href={normalized}>{label}</a>
          );
        },
      }}
    >
      {children}
    </ReactMarkdown>
  );
}
