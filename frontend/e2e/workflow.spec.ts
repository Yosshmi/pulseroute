import { test, expect } from "@playwright/test";
import process from "node:process";
test("register, create a project and endpoint, ingest and inspect delivery", async ({
  page,
}) => {
  await page.goto("/");
  await page
    .getByRole("button", { name: "New here? Create an account" })
    .click();
  await page
    .getByLabel("Email", { exact: true })
    .fill(`browser-${Date.now()}@pulseroute.test`);
  await page
    .getByLabel("Password", { exact: true })
    .fill("browser-test-password");
  await page
    .getByRole("button", { name: "Create account", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Delivery overview" }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Projects", exact: true }).click();
  await page.getByLabel("Project name").fill("Browser workflow");
  await page
    .getByRole("button", { name: "Create project", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Browser workflow" }),
  ).toBeVisible();
  await page.getByLabel("Name", { exact: true }).fill("Browser receiver");
  await page
    .getByLabel("Destination URL")
    .fill(process.env.E2E_RECEIVER_URL ?? "http://127.0.0.1:8090");
  await page
    .getByLabel("Signing secret (blank generates one)")
    .fill("browser-receiver-secret");
  await page.getByRole("button", { name: "Add endpoint", exact: true }).click();
  await expect(
    page.getByText("Browser receiver", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("link", { name: "API keys", exact: true })
    .first()
    .click();
  await page
    .getByRole("combobox", { name: "Project", exact: true })
    .selectOption({ label: "Browser workflow" });
  await page.getByLabel("Key name").fill("Browser key");
  await page.getByRole("button", { name: "Create API key" }).click();
  const key = await page.locator(".secret code").textContent();
  expect(key).toBeTruthy();
  const result = await page.request.post("/v1/events", {
    headers: { Authorization: `Bearer ${key}` },
    data: {
      event_id: `browser-event-${Date.now()}`,
      type: "order.created",
      data: { synthetic: true },
    },
  });
  expect(result.status()).toBe(202);
  await page.getByRole("link", { name: "Deliveries", exact: true }).click();
  await expect(
    page.getByText("Browser receiver", { exact: true }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Inspect →" }).first().click();
  await expect(
    page.getByRole("heading", { name: "Attempt timeline" }),
  ).toBeVisible();
  await expect(async () => {
    await page.getByRole("button", { name: "Refresh", exact: true }).click();
    await expect(page.locator(".delivery-summary .badge")).toHaveText(
      "succeeded",
    );
  }).toPass({ timeout: 15000 });
  await page.screenshot({
    path: "test-results/delivery-desktop.png",
    fullPage: true,
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({
    path: "test-results/delivery-mobile.png",
    fullPage: true,
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBeTruthy();
});
