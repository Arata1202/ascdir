"use client";

import { useEffect, useState } from "react";

type TocItem = { id: string; title: string };

export function DocToc({ items }: { items: TocItem[] }) {
  const [activeId, setActiveId] = useState(items[0]?.id ?? "");

  useEffect(() => {
    const headings = items
      .map((item) => document.getElementById(item.id))
      .filter((heading): heading is HTMLElement => heading !== null);
    if (headings.length === 0) return;

    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((entry) => entry.isIntersecting)
          .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (visible[0]?.target.id) setActiveId(visible[0].target.id);
      },
      { rootMargin: "-15% 0px -70% 0px" },
    );

    headings.forEach((heading) => observer.observe(heading));
    return () => observer.disconnect();
  }, [items]);

  return (
    <nav aria-label="On this page">
      <p>On this page</p>
      {items.map((item) => (
        <a
          href={`#${item.id}`}
          aria-current={activeId === item.id ? "location" : undefined}
          key={item.id}
        >
          {item.title}
        </a>
      ))}
    </nav>
  );
}
