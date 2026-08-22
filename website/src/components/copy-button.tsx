"use client";

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

export function CopyButton({
  value,
  label,
  variant = "text",
  className = "",
}: {
  value: string | (() => string);
  label: string;
  variant?: "text" | "icon";
  className?: string;
}) {
  const [status, setStatus] = useState<CopyStatus>("idle");
  const resetTimer = useRef<number | undefined>(undefined);

  useEffect(() => () => window.clearTimeout(resetTimer.current), []);

  async function copy() {
    const content = (typeof value === "function" ? value() : value).trimEnd();
    try {
      if (navigator.clipboard?.writeText) await navigator.clipboard.writeText(content);
      else legacyCopy(content);
      setStatus("copied");
    } catch {
      try {
        legacyCopy(content);
        setStatus("copied");
      } catch {
        setStatus("error");
      }
    }
    window.clearTimeout(resetTimer.current);
    resetTimer.current = window.setTimeout(() => setStatus("idle"), 1800);
  }

  const accessibleLabel =
    status === "copied" ? `${label}: copied` : status === "error" ? `${label}: copy failed` : label;

  return (
    <>
      <button
        className={`copyButton ${className}`.trim()}
        type="button"
        onClick={copy}
        aria-label={accessibleLabel}
      >
        {variant === "icon" ? (
          status === "copied" ? (
            <svg aria-hidden="true" viewBox="0 0 20 20" width="17" height="17">
              <path
                d="m4 10 3.4 3.4L16 5.8"
                fill="none"
                stroke="currentColor"
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth="1.7"
              />
            </svg>
          ) : (
            <svg aria-hidden="true" viewBox="0 0 20 20" width="17" height="17">
              <rect
                x="6.5"
                y="3.5"
                width="9"
                height="11"
                rx="1.8"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.4"
              />
              <path
                d="M4.5 6.5h-1v10h8v-1"
                fill="none"
                stroke="currentColor"
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth="1.4"
              />
            </svg>
          )
        ) : status === "copied" ? (
          "Copied"
        ) : status === "error" ? (
          "Try again"
        ) : (
          "Copy"
        )}
      </button>
      <span className="visuallyHidden" role="status" aria-live="polite">
        {status === "copied"
          ? "Code copied to clipboard"
          : status === "error"
            ? "Could not copy code"
            : ""}
      </span>
    </>
  );
}
