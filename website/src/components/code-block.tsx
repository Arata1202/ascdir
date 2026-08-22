"use client";

import type { ReactNode, UIEvent } from "react";
import { useEffect, useRef, useState } from "react";
import { CopyButton } from "./copy-button";

export function CodeBlock({ children }: { children: ReactNode }) {
  const preRef = useRef<HTMLPreElement>(null);
  const [showsScrollCue, setShowsScrollCue] = useState(false);

  function updateScrollCue() {
    const pre = preRef.current;
    if (!pre) return;
    setShowsScrollCue(pre.scrollLeft + pre.clientWidth < pre.scrollWidth - 1);
  }

  useEffect(() => {
    updateScrollCue();
    const pre = preRef.current;
    if (!pre) return;
    const observer = new ResizeObserver(updateScrollCue);
    observer.observe(pre);
    return () => observer.disconnect();
  }, []);

  function handleScroll(_event: UIEvent<HTMLPreElement>) {
    updateScrollCue();
  }

  return (
    <div className={`codeBlock${showsScrollCue ? " codeBlockScrollable" : ""}`}>
      <pre ref={preRef} onScroll={handleScroll}>
        {children}
      </pre>
      {showsScrollCue ? <span className="codeScrollCue" aria-hidden="true" /> : null}
      <CopyButton value={() => preRef.current?.textContent ?? ""} label="Copy code to clipboard" />
    </div>
  );
}
