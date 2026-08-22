import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { absoluteUrl, isCloudflarePreview, pageMetadata, site } from "../src/lib/site";

describe("site URLs", () => {
  it("builds canonical absolute URLs", () => {
    expect(absoluteUrl("/docs/")).toBe(`${site.url}/docs/`);
  });

  it("builds complete page-specific social metadata", () => {
    const metadata = pageMetadata({
      title: "Documentation",
      description: "Official documentation.",
      path: "/docs/",
    });
    expect(metadata.alternates).toEqual({ canonical: "/docs/" });
    expect(metadata.openGraph).toMatchObject({
      title: "Documentation",
      url: "/docs/",
      images: [{ url: "/og.png", width: 1280, height: 640 }],
    });
    expect(metadata.twitter).toMatchObject({ title: "Documentation", images: ["/og.png"] });
  });

  it("only disables indexing for Cloudflare preview branches", () => {
    expect(isCloudflarePreview({ CF_PAGES: "1", CF_PAGES_BRANCH: "feature" })).toBe(true);
    expect(isCloudflarePreview({ CF_PAGES: "1", CF_PAGES_BRANCH: "main" })).toBe(false);
    expect(isCloudflarePreview({})).toBe(false);
  });

  it("ships baseline Cloudflare security headers", () => {
    const headers = fs.readFileSync(path.resolve(process.cwd(), "public/_headers"), "utf8");
    expect(headers).toContain("Content-Security-Policy:");
    expect(headers).toContain("frame-ancestors 'none'");
    expect(headers).toContain("X-Content-Type-Options: nosniff");
    expect(headers).toContain("Permissions-Policy:");
  });
});
