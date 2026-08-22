import { describe, expect, it } from "vitest";
import { docMeta, getDoc, getDocSlugs } from "../src/lib/docs";

describe("documentation source", () => {
  it("exposes stable, unique slugs", async () => {
    const slugs = await getDocSlugs();
    expect(slugs).toContain("getting-started");
    expect(new Set(slugs).size).toBe(slugs.length);
    expect(Object.keys(docMeta).sort()).toEqual(slugs);
  });

  it("derives metadata from each document", async () => {
    for (const slug of await getDocSlugs()) {
      const doc = await getDoc(slug);
      expect(doc?.title.length).toBeGreaterThan(2);
      expect(doc?.description.length).toBeGreaterThan(10);
      expect(doc?.body).not.toMatch(/^#\s/);
      expect(new Set(doc?.toc.map((item) => item.id)).size).toBe(doc?.toc.length);
    }
  });

  it("rejects unsafe paths", async () => {
    await expect(getDoc("../README")).resolves.toBeNull();
  });
});
