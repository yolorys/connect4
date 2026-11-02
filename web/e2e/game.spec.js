import { test, expect } from '@playwright/test';

async function expectDiscCount(page, count) {
  await expect(page.locator('.cell[data-player="1"], .cell[data-player="2"]')).toHaveCount(count);
}

async function start(page) {
  await page.goto('/');
  await page.getByRole('button', { name: 'Start game' }).click();
  await expect(page.getByRole('heading', { name: 'Red’s turn' })).toBeVisible();
}

async function drop(page, column, count) {
  const result = page.waitForResponse((response) => response.url().endsWith('/moves') && response.request().method() === 'POST');
  await page.getByRole('button', { name: `Drop in column ${column + 1}`, exact: true }).click();
  expect((await result).status()).toBe(200);
  await expectDiscCount(page, count);
}

test('gravity, turn changes, highlight, refresh, and a new game', async ({ page }) => {
  await start(page);
  await drop(page, 3, 1);
  await expect(page.locator('[data-last="true"]')).toHaveAttribute('data-row', '5');
  await expect(page.getByRole('heading', { name: 'Yellow’s turn' })).toBeVisible();
  await drop(page, 3, 2);
  await expect(page.locator('[data-last="true"]')).toHaveCount(1);
  await expect(page.locator('[data-last="true"]')).toHaveAttribute('data-row', '4');
  const id = await page.evaluate(() => localStorage.getItem('connect4.gameId'));
  await page.reload();
  await expectDiscCount(page, 2);
  await expect(page.locator('[data-last="true"]')).toHaveAttribute('data-row', '4');
  await expect(page.getByRole('heading', { name: 'Red’s turn' })).toBeVisible();
  await page.getByRole('button', { name: 'New game' }).click();
  await expectDiscCount(page, 0);
  await expect(page.locator('[data-last="true"]')).toHaveCount(0);
  expect(await page.evaluate(() => localStorage.getItem('connect4.gameId'))).not.toBe(id);
});

test('a full column is disabled', async ({ page }) => {
  await start(page);
  for (let i = 0; i < 6; i++) await drop(page, 2, i + 1);
  await expect(page.getByRole('button', { name: 'Column 3 is full' })).toBeDisabled();
  await expect(page.getByRole('button', { name: 'Drop in column 4', exact: true })).toBeEnabled();
});

test('a win ends play and survives refresh', async ({ page }) => {
  await start(page);
  for (const [i, column] of [0,1,0,1,0,1,0].entries()) await drop(page, column, i + 1);
  await expect(page.getByRole('heading', { name: 'Red wins!' })).toBeVisible();
  await expect(page.locator('.column:enabled')).toHaveCount(0);
  await page.reload();
  await expect(page.getByRole('heading', { name: 'Red wins!' })).toBeVisible();
  await expect(page.locator('[data-last="true"]')).toHaveCount(1);
});

test('a full board without a winner is a draw', async ({ page }) => {
  await start(page);
  const moves = [6,6,6,5,2,2,5,1,4,3,3,6,3,4,0,4,1,0,4,4,1,6,5,1,0,5,1,4,3,1,5,3,0,6,5,2,2,0,2,3,2,0];
  for (const [i, column] of moves.entries()) await drop(page, column, i + 1);
  await expect(page.getByRole('heading', { name: 'It’s a draw.' })).toBeVisible();
  await expect(page.locator('.column:enabled')).toHaveCount(0);
});

test('a stale tab reloads without adding an unintended move', async ({ page }) => {
  await start(page);
  const other = await page.context().newPage();
  await other.goto('/');
  await expect(other.getByRole('heading', { name: 'Red’s turn' })).toBeVisible();
  await expectDiscCount(other, 0);
  await drop(page, 0, 1);
  await other.getByRole('button', { name: 'Drop in column 2', exact: true }).click();
  await expect(other.getByRole('alert')).toContainText('The board changed.');
  await expectDiscCount(other, 1);
  await expect(other.locator('[data-row="5"][data-column="1"]')).toHaveAttribute('data-player', '0');
  await drop(other, 1, 2);
});

test('a lost response is recovered without resending the move', async ({ page }) => {
  await start(page);
  await page.route('**/api/games/*/moves', async (route) => {
    const response = await route.fetch();
    expect(response.status()).toBe(200);
    await route.abort('failed');
  });
  await page.getByRole('button', { name: 'Drop in column 4', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('could not confirm');
  await expect(page.locator('.column:enabled')).toHaveCount(0);
  await page.unroute('**/api/games/*/moves');
  await page.getByRole('button', { name: 'Reload game' }).click();
  await expectDiscCount(page, 1);
  await expect(page.getByRole('heading', { name: 'Yellow’s turn' })).toBeVisible();
  await expect(page.locator('[data-last="true"]')).toHaveAttribute('data-column', '3');
});

test('the board fits a small screen and supports keyboard play', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 780 });
  await start(page);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  const column = page.getByRole('button', { name: 'Drop in column 1', exact: true });
  await column.focus();
  await page.keyboard.press('Enter');
  await expectDiscCount(page, 1);
});
