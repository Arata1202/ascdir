import Link from "next/link";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { CodeBlock } from "./code-block";

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
