"use client";

import type { ReactNode } from "react";
import { useRef } from "react";
import { CopyButton } from "./copy-button";

export function CodeBlock({ children }: { children: ReactNode }) {
  const preRef = useRef<HTMLPreElement>(null);

  return (
    <div className="codeBlock">
      <pre ref={preRef}>{children}</pre>
      <CopyButton value={() => preRef.current?.textContent ?? ""} label="Copy code to clipboard" />
    </div>
  );
}
