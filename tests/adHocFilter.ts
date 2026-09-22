import {expect, Page} from "@playwright/test";
import {pickOptionByExactText, pickOptionByPrefix, visibleOptionTexts} from "./grafanaSelect";

/**
 * Page-object for Grafana's dashboard "Ad hoc filters" variable (same shape
 * as {@link VariablePicker} / {@link QueryEditorRow}).
 *
 * Every supported Grafana renders this variable as one combobox: a single
 * text input driving successive key → operator → value listboxes, with
 * `getTagValues` issued when the operator is picked. Only the placeholder
 * moved across the matrix, so both spellings are matched here and share one
 * code path.
 *
 * Probed on the dev stack (2026-08-26) rather than inferred — 11.5.4, 12.0.2
 * and 12.3.1 render placeholder "Filter by label values"; 13.0.1 renders
 * "+ label = value".
 *
 * The methods are expressed in terms of what a *test* wants ("open the value
 * dropdown for `status`") rather than which widget is clicked, so the specs
 * stay version-agnostic.
 *
 * Option matching goes through {@link pickOptionByExactText} rather than
 * `getByRole("option", {name})` because `status` would also prefix-match
 * `status_null` / `status_nonnull`.
 *
 * If you ever need to probe for this variable structurally, do not use a
 * `[data-testid^="AdHocFilter"]` prefix match: 13.x renders an
 * `AdHocFilter-label-announcer` live-region as soon as a filter is committed,
 * so the prefix form matches on a page that has no filter widget in reach.
 */

/**
 * The combobox input. Its placeholder was rewritten between the 11.x–12.x
 * variable ("Filter by label values") and the 13.x combobox
 * ("+ label = value"); both drive the same key → operator → value listbox
 * sequence.
 */
const COMBOBOX = [
    'input[placeholder="+ label = value"]',
    'input[placeholder="Filter by label values"]',
].join(", ");

export class AdHocFilter {
    constructor(private readonly page: Page) {}

    /**
     * Select `key` for a new filter, stopping just short of loading its
     * values.
     */
    async selectKey(key: string): Promise<void> {
        await this.page.locator(COMBOBOX).first().click();
        await pickOptionByExactText(this.page, key);
        // The value listbox only opens once an operator is chosen. Option
        // names concatenate label+description, hence the prefix match.
        await pickOptionByPrefix(this.page, "=Equals");
    }

    /**
     * Open the value dropdown and wait for the preload to land.
     *
     * `elapsedMs` is measured from entry. The click that actually issues
     * `getTagValues` is the operator pick, which already happened in
     * {@link selectKey}, so the listbox is opening as we arrive and the
     * measurement covers the request's remaining flight time — which is what
     * the guardrail-budget assertions care about.
     *
     * Set `waitForOptions: false` when the preload is *expected* to come back
     * empty (e.g. a dashboard range deliberately pinned away from the fixture
     * data). Waiting for a non-empty list would otherwise hang until timeout,
     * and callers in that situation assert on the captured request rather
     * than on what the dropdown renders.
     */
    async openValues(
        opts: { timeout?: number; waitForOptions?: boolean } = {},
    ): Promise<{ options: string[]; elapsedMs: number }> {
        const timeout = opts.timeout ?? 8000;
        const waitForOptions = opts.waitForOptions ?? true;
        const start = Date.now();

        await this.page.waitForSelector('[role="listbox"]', {timeout});
        if (waitForOptions) {
            // A single "Loading options..." placeholder is parked in the
            // listbox while the request is in flight; real options replace it.
            await expect
                .poll(
                    async () => {
                        const opts_ = await visibleOptionTexts(this.page);
                        return opts_.length > 0 && !opts_.some((o) => o.startsWith("Loading"));
                    },
                    {timeout, intervals: [50, 100, 200]},
                )
                .toBe(true);
        }

        return {
            options: await visibleOptionTexts(this.page),
            elapsedMs: Date.now() - start,
        };
    }

    /**
     * Re-open the value dropdown of the filter built by the preceding
     * {@link selectKey} / {@link openValues} pair, issuing a *second*
     * `getTagValues`. Used to prove repeated opens produce identical SQL.
     *
     * The combobox discards its in-progress filter on Escape, so the whole
     * key+operator walk has to be replayed.
     */
    async reopenValues(
        key: string,
        opts: { timeout?: number; waitForOptions?: boolean } = {},
    ): Promise<{ options: string[]; elapsedMs: number }> {
        await this.selectKey(key);
        return this.openValues(opts);
    }

    /** Commit `value` from the open value dropdown, completing the filter. */
    async pickValue(value: string): Promise<void> {
        await pickOptionByExactText(this.page, value);
    }

    /**
     * Type a value that the preload deliberately does not suggest and commit
     * it, proving manual entry still reaches the query. The open dropdown's
     * text input is exposed as the expanded combobox.
     */
    async typeValue(value: string): Promise<void> {
        const input = this.page
            .locator('[role="combobox"][aria-expanded="true"]')
            .last();
        await input.pressSequentially(value);
        await this.page.waitForTimeout(300);
        await input.press("Enter");
    }

    /** Abandon the dropdown currently open, discarding the in-progress filter. */
    async dismiss(): Promise<void> {
        await this.page.keyboard.press("Escape");
    }
}
