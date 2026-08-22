import { expect, test } from "@playwright/test";

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
  const headingCode = page.getByRole("heading", { level: 2, name: "ascdir.yaml not found" }).locator("code");
  await expect(headingCode).toHaveCSS("background-color", "rgba(0, 0, 0, 0)");
});

test("mobile navigation keeps the primary pages discoverable", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await page.getByLabel("Open navigation").click();
  await expect(page.getByRole("link", { name: "Docs" }).last()).toBeVisible();
  await expect(page.getByRole("link", { name: "Comparison" }).last()).toBeVisible();
  await expect(page.getByRole("link", { name: "Changelog" }).last()).toBeVisible();
});

test("comparison remains complete without horizontal scrolling on mobile", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/comparison/fastlane/");

  const comparison = page.getByLabel("ascdir and fastlane comparison").filter({ visible: true });
  await expect(comparison.getByText("Primary focus")).toBeVisible();
  await expect(comparison.getByText("End-to-end mobile release automation")).toBeVisible();
  await expect(comparison.getByText("HTML metadata preview in deliver; behavior varies by action")).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(390);
});
