import {Locator, Page} from "@playwright/test";

/**
 * Cross-version helpers for Grafana's react-select widgets.
 *
 * Grafana wraps every react-select in a `<div data-value="">` that owns the
 * click handler and intercepts pointer events on the visible value text.
 * Clicking the text/combobox directly fails with "intercepts pointer events";
 * the `[data-value=""]` wrapper is what must be clicked.
 *
 * The option list renders in a portal at the document root and its role
 * differs by Grafana version:
 *   - 11.x–13.x: `role="option"` inside a portal listbox.
 *   - 10.x: variable pickers render as `role="checkbox"` inside a list;
 *           plugin-owned react-selects (querySettings) still use
 *           `role="option"`. The `.or()` chain accepts either.
 *
 * The dropdown option's accessible name also differs by version:
 *   - 11.x+: react-select concatenates label + description into the
 *            accessible name with a space separator.
 *   - 10.x: the option's accessible name is the constant "Select option";
 *           the label appears only as inner text, concatenated with the
 *           description without whitespace.
 * Pick by name when matching exact short values; pick by prefix when the
 * value is a known leading substring (no `\b` — there's no word boundary
 * between the label and the description on 10.x).
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
    await page
        .getByRole("option", {name})
        .or(page.getByRole("checkbox", {name}))
        .first()
        .click();
}

/**
 * Pick an option whose inner text starts with `prefix`. Use when the option's
 * accessible name varies by version (see file-level note).
 */
export async function pickOptionByPrefix(
    page: Page,
    prefix: string,
): Promise<void> {
    const re = new RegExp(`^${escapeRegex(prefix)}`);
    await page
        .getByRole("option").filter({hasText: re})
        .or(page.getByRole("checkbox", {name: re}))
        .first()
        .click();
}
