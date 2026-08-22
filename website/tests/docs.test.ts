import { describe, expect, it } from "vitest";
import { getDoc, getDocSlugs } from "../src/lib/docs";

describe("documentation source", () => {
  it("exposes stable, unique slugs", async () => {
    const slugs = await getDocSlugs();
    expect(slugs).toContain("getting-started");
    expect(new Set(slugs).size).toBe(slugs.length);
  });

  it("derives metadata from each document", async () => {
    for (const slug of await getDocSlugs()) {
      const doc = await getDoc(slug);
      expect(doc?.title.length).toBeGreaterThan(2);
      expect(doc?.description.length).toBeGreaterThan(10);
      expect(doc?.body).not.toMatch(/^#\s/);
    }
  });

  it("rejects unsafe paths", async () => {
    await expect(getDoc("../README")).resolves.toBeNull();
  });
});
