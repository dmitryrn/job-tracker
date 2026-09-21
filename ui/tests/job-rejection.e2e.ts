import { expect, test } from "@playwright/test";

test("opens the won't apply dialog from the job actions menu", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 900 });
  await page.goto("/jobs/1921/match");

  const menu = page.locator("details.job-overflow");
  await menu.locator("summary").click();

  const rejectAction = menu.locator(".job-overflow-menu > .reject-action");
  await expect(rejectAction).toBeVisible();
  await rejectAction.click();

  const dialog = page.locator("dialog.job-rejection-dialog[open]");
  await expect(dialog).toBeVisible();
  await expect(dialog.getByLabel("Reason")).toBeEditable();
  const dimensions = await dialog.evaluate((element) => ({ clientWidth: element.clientWidth, scrollWidth: element.scrollWidth }));
  expect(dimensions.scrollWidth).toBeLessThanOrEqual(dimensions.clientWidth);

  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).toBeHidden();
  await expect(menu).not.toHaveAttribute("open", "");

  await menu.locator("summary").click();
  await expect(menu.locator(".job-overflow-menu")).toBeVisible();
});
