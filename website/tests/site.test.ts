import { describe, expect, it } from "vitest";
import { absoluteUrl, site } from "../src/lib/site";

describe("site URLs", () => {
  it("builds canonical absolute URLs", () => {
    expect(absoluteUrl("/docs/")).toBe(`${site.url}/docs/`);
  });
});
