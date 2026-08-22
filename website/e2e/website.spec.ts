import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { getDocSlugs } from "../src/lib/docs";

test("home page provides a clear route into documentation", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 })).toContainText("reviewed before it ships");
  await expect(page.getByRole("link", { name: /get started/i })).toHaveAttribute(
    "href",
    "/docs/getting-started/",
  );
  const installCopy = page.getByRole("button", { name: "Copy install command" });
  await expect(installCopy).toHaveCSS("height", "44px");
  await installCopy.click();
  await expect(page.getByRole("status")).toHaveText("Code copied to clipboard");
});

test("reduced-motion visitors receive a static product preview", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/");
  await expect(page.locator(".demoMotion")).toBeHidden();
  await expect(page.locator(".demoStill")).toBeVisible();
});

test("documentation is statically navigable", async ({ page }) => {
  await page.goto("/docs/");
  await page
    .getByRole("link", { name: /getting started/i })
    .first()
    .click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Getting started");
  await expect(page.getByRole("button", { name: "Copy code to clipboard" }).first()).toBeVisible();
  await expect(page.getByRole("navigation", { name: "Documentation pages" })).toBeVisible();
  await expect(page.getByRole("navigation", { name: "On this page" })).toBeVisible();
  await page.getByRole("button", { name: "Copy code to clipboard" }).first().click();
  await expect(page.getByRole("status").first()).toHaveText("Code copied to clipboard");
  const credentialsLink = page
    .getByRole("navigation", { name: "On this page" })
    .getByRole("link", { name: "Configure App Store Connect credentials" });
  await credentialsLink.click();
  await expect(credentialsLink).toHaveAttribute("aria-current", "location");
  const credentialsHeading = page.getByRole("heading", {
    level: 2,
    name: "Configure App Store Connect credentials",
  });
  await expect
    .poll(() => credentialsHeading.evaluate((heading) => heading.getBoundingClientRect().top))
    .toBeGreaterThan(52);
});

test("mobile documentation keeps long code and headings readable", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/docs/getting-started/");
  const overflowingBlock = page.locator(".codeBlockScrollable").first();
  await expect(overflowingBlock.locator(".codeScrollCue")).toBeVisible();
  await overflowingBlock.locator("pre").evaluate((pre) => pre.scrollTo({ left: pre.scrollWidth }));
  await expect(overflowingBlock.locator(".codeScrollCue")).toBeHidden();

  await page.goto("/docs/troubleshooting/");
  const headingCode = page
    .getByRole("heading", { level: 2, name: "ascdir.yaml not found" })
    .locator("code");
  await expect(headingCode).toHaveCSS("background-color", "rgba(0, 0, 0, 0)");
});

test("mobile navigation keeps the primary pages discoverable", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await page.getByLabel("Open navigation").click();
  await expect(page.getByLabel("Close navigation")).toHaveAttribute("aria-expanded", "true");
  await expect(page.getByRole("link", { name: "Docs" }).last()).toBeVisible();
  await expect(page.getByRole("link", { name: "Comparison" }).last()).toBeVisible();
  await expect(page.getByRole("link", { name: "Changelog" }).last()).toBeVisible();
  await page.getByRole("link", { name: "Docs" }).last().click();
  await expect(page).toHaveURL(/\/docs\/$/);
  await expect(page.getByLabel("Open navigation")).toHaveAttribute("aria-expanded", "false");
});

test("comparison remains complete without horizontal scrolling on mobile", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/comparison/fastlane/");

  const comparison = page.getByLabel("ascdir and fastlane comparison").filter({ visible: true });
  await expect(comparison.getByText("Primary focus")).toBeVisible();
  await expect(comparison.getByText("End-to-end mobile release automation")).toBeVisible();
  await expect(
    comparison.getByText("HTML metadata preview in deliver; behavior varies by action"),
  ).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(390);
});

test("changelog has a clear page hierarchy without empty release sections", async ({ page }) => {
  await page.goto("/changelog/");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Changelog");
  await expect(page.getByRole("heading", { name: "Unreleased" })).toHaveCount(0);
  await expect(page.getByRole("heading", { level: 2 }).first()).toContainText("1.2.1");
});

test("all static pages have complete metadata and no mobile overflow", async ({ page }) => {
  const routes = [
    "/",
    "/docs/",
    "/comparison/fastlane/",
    "/changelog/",
    ...(await getDocSlugs()).map((slug) => `/docs/${slug}/`),
  ];
  const internalLinks = new Set<string>();

  for (const route of routes) {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(route);
    await expect(page.locator("h1")).toHaveCount(1);
    await expect(page.locator('link[rel="canonical"]')).toHaveAttribute("href", /https:\/\//);
    await expect(page.locator('meta[property="og:title"]')).toHaveAttribute("content", /.+/);
    await expect(page.locator('meta[property="og:image"]')).toHaveAttribute("content", /og\.png$/);
    const validJsonLd = await page
      .locator('script[type="application/ld+json"]')
      .evaluateAll((scripts) =>
        scripts.every((script) => {
          try {
            JSON.parse(script.textContent ?? "");
            return true;
          } catch {
            return false;
          }
        }),
      );
    expect(validJsonLd).toBe(true);
    for (const href of await page
      .locator('a[href^="/"]')
      .evaluateAll((anchors) => anchors.map((anchor) => anchor.getAttribute("href")))) {
      if (href) internalLinks.add(href);
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(390);
  }

  for (const href of internalLinks) {
    const response = await page.request.get(href);
    expect(response.ok(), `${href} returned ${response.status()}`).toBe(true);
  }
});

test("representative page templates have no automated accessibility violations", async ({
  page,
}) => {
  for (const route of [
    "/",
    "/docs/",
    "/docs/getting-started/",
    "/comparison/fastlane/",
    "/changelog/",
  ]) {
    await page.goto(route);
    const result = await new AxeBuilder({ page }).analyze();
    expect(result.violations, `${route}: ${JSON.stringify(result.violations)}`).toEqual([]);
  }
});

test("the generated 404 page is not indexable", async ({ page }) => {
  const response = await page.goto("/missing-page/");
  expect(response?.status()).toBe(404);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("This page does not exist.");
  const robots = await page
    .locator('meta[name="robots"]')
    .evaluateAll((elements) => elements.map((element) => element.getAttribute("content") ?? ""));
  expect(robots.join(",")).not.toContain("index, follow");
});
