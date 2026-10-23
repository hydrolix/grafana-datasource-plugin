import {Locator, Page} from "@playwright/test";

/**
 * Cross-version helpers for Grafana's react-select widgets.
 *
 * Grafana wraps every react-select in a `<div data-value="">` that owns the
 * click handler and intercepts pointer events on the visible value text.
 * Clicking the text/combobox directly fails with "intercepts pointer events";
 * the `[data-value=""]` wrapper is what must be clicked.
 *
 * The option list renders in a portal at the document root as
 * `role="option"` inside a portal listbox, and react-select concatenates
 * label + description into the accessible name with a space separator.
 *
 * Pick by name when matching exact short values; pick by prefix when the
 * value is a known leading substring.
 */

function escapeRegex(s: string): string {
    return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/**
 * Open a Grafana react-select scoped under `root`. Uses `.last()` to disambiguate
 * when the panel-editor right pane introduces additional `[data-value=""]`
 * surfaces; callers must scope `root` themselves (e.g. to a query row).
 */
export async function openGrafanaSelect(root: Locator): Promise<void> {
    await root.locator('[data-value=""]').last().click();
}

/**
 * Pick an option by exact accessible name. Page-scoped because the option
 * list lives in a portal at the document root, never inside the picker's
 * subtree.
 */
export async function pickOption(page: Page, name: string): Promise<void> {
    await page.getByRole("option", {name}).first().click();
}

/**
 * Pick an option whose inner text starts with `prefix`. Use when the option's
 * accessible name varies by version (see file-level note).
 *
 * The anchor tolerates leading whitespace: 12.x indents the option's markup,
 * so its raw text content starts with a newline and a bare `^` anchor matches
 * nothing at all — the option is on screen and visible, but `hasText` reports
 * zero matches.
 */
export async function pickOptionByPrefix(
    page: Page,
    prefix: string,
): Promise<void> {
    const re = new RegExp(`^\\s*${escapeRegex(prefix)}`);
    await page.getByRole("option").filter({hasText: re}).first().click();
}

/**
 * Locate an option by its exact *inner text*, ignoring the accessible name.
 *
 * Prefer this over {@link pickOption} whenever the option label is itself a
 * prefix of another label in the same list (`status` vs `status_null`).
 * Values are regex-escaped, so labels containing metacharacters
 * (`attrs['env']`) are safe to pass verbatim.
 */
export function optionByExactText(page: Page, text: string): Locator {
    const re = new RegExp(`^\\s*${escapeRegex(text)}\\s*$`);
    return page.getByRole("option").filter({hasText: re});
}

/** Click the option whose inner text is exactly `text`. */
export async function pickOptionByExactText(
    page: Page,
    text: string,
): Promise<void> {
    await optionByExactText(page, text).first().click();
}

/** Trimmed inner text of every currently rendered option. */
export async function visibleOptionTexts(page: Page): Promise<string[]> {
    const texts = await page.getByRole("option").allTextContents();
    return texts.map((t) => t.trim());
}
