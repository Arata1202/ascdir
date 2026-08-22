import { expect, test } from "@playwright/test";

test("home page provides a clear route into documentation", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 })).toContainText("reviewed before it runs");
  await expect(page.getByRole("link", { name: /get started/i })).toHaveAttribute(
    "href",
    "/docs/getting-started/",
  );
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
});

test("mobile navigation keeps the primary pages discoverable", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await page.getByLabel("Open navigation").click();
  await expect(page.getByRole("link", { name: "Docs" }).last()).toBeVisible();
  await expect(page.getByRole("link", { name: "Comparison" }).last()).toBeVisible();
  await expect(page.getByRole("link", { name: "Changelog" }).last()).toBeVisible();
});
