"use client";

import type { ReactNode } from "react";
import { useEffect, useRef, useState } from "react";

type CopyStatus = "idle" | "copied" | "error";

function legacyCopy(value: string) {
  const input = document.createElement("textarea");
  input.value = value;
  input.setAttribute("readonly", "");
  input.style.position = "fixed";
  input.style.opacity = "0";
  document.body.appendChild(input);
  input.select();
  const copied = document.execCommand("copy");
  input.remove();
  if (!copied) throw new Error("Copy command was rejected");
}

export function CodeBlock({ children }: { children: ReactNode }) {
  const preRef = useRef<HTMLPreElement>(null);
  const [status, setStatus] = useState<CopyStatus>("idle");
  const resetTimer = useRef<number | undefined>(undefined);

  useEffect(() => () => window.clearTimeout(resetTimer.current), []);

  async function copy() {
    const value = preRef.current?.textContent ?? "";
    const code = value.trimEnd();
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(code);
      } else {
        legacyCopy(code);
      }
      setStatus("copied");
    } catch {
      try {
        legacyCopy(code);
        setStatus("copied");
      } catch {
        setStatus("error");
      }
    }
    window.clearTimeout(resetTimer.current);
    resetTimer.current = window.setTimeout(() => setStatus("idle"), 1800);
  }

  return (
    <div className="codeBlock">
      <pre ref={preRef}>{children}</pre>
      <button
        type="button"
        onClick={copy}
        aria-label={status === "error" ? "Copy failed" : "Copy code to clipboard"}
      >
        {status === "copied" ? "Copied" : status === "error" ? "Try again" : "Copy"}
      </button>
      <span className="visuallyHidden" role="status" aria-live="polite">
        {status === "copied"
          ? "Code copied to clipboard"
          : status === "error"
            ? "Could not copy code"
            : ""}
      </span>
    </div>
  );
}
