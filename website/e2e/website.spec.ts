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
});
