"use client";

import type { ReactNode } from "react";
import { useRef, useState } from "react";

export function CodeBlock({ children }: { children: ReactNode }) {
  const preRef = useRef<HTMLPreElement>(null);
  const [copied, setCopied] = useState(false);

  async function copy() {
    const value = preRef.current?.textContent ?? "";
    await navigator.clipboard.writeText(value.replace(/Copy$|Copied$/u, "").trimEnd());
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  }

  return (
    <div className="codeBlock">
      <pre ref={preRef}>{children}</pre>
      <button type="button" onClick={copy} aria-label="Copy code to clipboard">
        {copied ? "Copied" : "Copy"}
      </button>
    </div>
  );
}
