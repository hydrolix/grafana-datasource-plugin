import {expect, Locator, Page} from "@playwright/test";
import {pickOptionByExactText, pickOptionByPrefix, visibleOptionTexts} from "./grafanaSelect";

/**
 * Cross-version page-object for Grafana's dashboard "Ad hoc filters"
 * variable (same shape as {@link VariablePicker} / {@link QueryEditorRow}).
 *
 * Grafana ships two structurally different renderers for this variable and
 * the plugin supports both (CI matrix: 10.4, 11.6, 12.0, 13.0):
 *
 * | | **combobox** (11.5+) | **segments** (10.4 only) |
 * |---|---|---|
 * | entry point | one text input (placeholder varies, see `COMBOBOX`) | `+` button, `aria-label="Add Filter"` |
 * | key / operator / value | successive listboxes on the *same* input | three separate segment buttons |
 * | operator | chosen from a listbox (`=Equals`) | pre-filled with `=`; no menu opens |
 * | what triggers `getTagValues` | picking the operator | clicking the value segment |
 *
 * The methods are expressed in terms of what a *test* wants ("open the value
 * dropdown for `status`") rather than which widget is clicked, so the specs
 * stay version-agnostic.
 *
 * Renderer per version, probed on the dev stack (2026-08-26) rather than
 * inferred — 10.4.16 segments; 11.5.4, 12.0.2 and 12.3.1 combobox with
 * placeholder "Filter by label values"; 13.0.1 combobox with placeholder
 * "+ label = value". The 10.4 segment DOM was cross-checked against a
 * Playwright trace from the 10.4.18 CI job.
 *
 * Option matching goes through {@link pickOptionByExactText} rather than
 * `getByRole("option", {name})`: on 10.x every option's accessible name is the
 * constant "Select option" (see `grafanaSelect.ts`), and `status` would also
 * prefix-match `status_null` / `status_nonnull`.
 */

export type AdHocUi = "combobox" | "segments";

/**
 * The single-input renderer. Its placeholder was rewritten between the
 * 11.5–12.3 variable ("Filter by label values") and the 13.x combobox
 * ("+ label = value"); both drive the same key → operator → value listbox
 * sequence, so they share a code path here.
 */
const COMBOBOX = [
    'input[placeholder="+ label = value"]',
    'input[placeholder="Filter by label values"]',
].join(", ");
/** Segment wrappers, present only on the legacy renderer. */
const ADD_KEY_SEGMENT = '[data-testid="AdHocFilterKey-add-key-wrapper"]';
const KEY_SEGMENT = '[data-testid="AdHocFilterKey-key-wrapper"]';
const VALUE_SEGMENT = '[data-testid="AdHocFilterValue-value-wrapper"]';

/**
 * Structural markers unique to the segment renderer. Deliberately an explicit
 * list rather than a `[data-testid^="AdHocFilter"]` prefix match: 13.x renders
 * an `AdHocFilter-label-announcer` live-region as soon as a filter exists, so
 * the prefix form reports "segments" on 13 the moment the first filter is
 * committed.
 */
const SEGMENT_MARKERS = [ADD_KEY_SEGMENT, KEY_SEGMENT, VALUE_SEGMENT].join(", ");

export class AdHocFilter {
    /**
     * Last renderer seen on this page, so mid-interaction probes stay
     * stable. See {@link detectUi} for why this is needed.
     */
    private lastSeenUi: AdHocUi | undefined;

    constructor(private readonly page: Page) {}

    /**
     * The 10.4 "+" button that starts a new filter. Matched by accessible
     * name, not `button[aria-label=…]`: the label is computed rather than a
     * literal attribute on the `<button>`, so the attribute selector finds
     * nothing.
     */
    private addFilterButton(): Locator {
        return this.page.getByRole("button", {name: "Add Filter"});
    }

    /**
     * Which renderer this Grafana uses. A DOM probe rather than a version
     * lookup: Grafana has gated the combobox behind a feature toggle, so the
     * version number is not a reliable proxy for what is on screen.
     *
     * Order matters. The combobox input is only present while *no* filter is
     * being built — picking a key replaces it — so a naive "is the combobox
     * there?" check flips to "segments" halfway through a combobox
     * interaction and sends the next step hunting for segment buttons that
     * will never exist. The segment renderer, by contrast, is identifiable at
     * every point in the interaction by its segment wrappers, which 13.x
     * never renders (verified on the dev stack, 2026-08-26). So: test for
     * segments first, then the combobox, then fall back to whatever this
     * page reported earlier.
     */
    async detectUi(): Promise<AdHocUi> {
        if ((await this.page.locator(SEGMENT_MARKERS).count()) > 0) {
            this.lastSeenUi = "segments";
        } else if ((await this.page.locator(COMBOBOX).count()) > 0) {
            this.lastSeenUi = "combobox";
        }
        return this.lastSeenUi ?? "combobox";
    }

    /** True when a filter row exists whose value has not been chosen yet. */
    private async hasIncompleteFilter(): Promise<boolean> {
        const seg = this.page.locator(VALUE_SEGMENT).getByRole("button");
        if ((await seg.count()) === 0) {
            return false;
        }
        return (await seg.last().innerText()).trim() === "Select value";
    }

    /**
     * Select `key` for a new filter, stopping just short of loading its
     * values.
     *
     * On the segment renderer a partially-built filter suppresses the "Add
     * Filter" button entirely, so a spec that abandons one dropdown (Escape)
     * and then asks for a different key would deadlock. When that happens the
     * key segment of the pending row is retargeted instead of starting a new
     * row — same observable intent, and it keeps the filter count from
     * growing.
     */
    async selectKey(key: string): Promise<void> {
        if ((await this.detectUi()) === "combobox") {
            await this.page.locator(COMBOBOX).first().click();
            await pickOptionByExactText(this.page, key);
            // The value listbox only opens once an operator is chosen. Option
            // names concatenate label+description, hence the prefix match.
            await pickOptionByPrefix(this.page, "=Equals");
            return;
        }

        if (await this.hasIncompleteFilter()) {
            await this.page.locator(KEY_SEGMENT).getByRole("button").last().click();
        } else {
            await this.addFilterButton().first().click();
        }
        await pickOptionByExactText(this.page, key);
        // Operator segment is pre-filled with "=" on this renderer.
    }

    /**
     * Open the value dropdown and wait for the preload to land.
     *
     * `elapsedMs` is measured from the click that actually issues
     * `getTagValues` — the operator pick on the combobox, the value segment
     * on the segment renderer — so guardrail-budget assertions mean the same
     * thing on both. On the combobox the operator pick already happened in
     * {@link selectKey}, so the listbox is opening as we arrive.
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

        if ((await this.detectUi()) === "segments") {
            await this.page.locator(VALUE_SEGMENT).getByRole("button").last().click();
        }

        await this.page.waitForSelector('[role="listbox"]', {timeout});
        if (waitForOptions) {
            // Both renderers park a single "Loading options..." placeholder in
            // the listbox while the request is in flight; real options replace
            // it.
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
     * key+operator walk is replayed; the segment renderer keeps the row, so
     * re-clicking its value segment is enough (and is what a user would do).
     */
    async reopenValues(
        key: string,
        opts: { timeout?: number; waitForOptions?: boolean } = {},
    ): Promise<{ options: string[]; elapsedMs: number }> {
        if ((await this.detectUi()) === "combobox") {
            await this.selectKey(key);
        }
        return this.openValues(opts);
    }

    /** Commit `value` from the open value dropdown, completing the filter. */
    async pickValue(value: string): Promise<void> {
        await pickOptionByExactText(this.page, value);
    }

    /**
     * Type a value that the preload deliberately does not suggest and commit
     * it, proving manual entry still reaches the query. Both renderers expose
     * the open dropdown's text input as the expanded combobox.
     */
    async typeValue(value: string): Promise<void> {
        const input = this.page
            .locator('[role="combobox"][aria-expanded="true"]')
            .last();
        await input.pressSequentially(value);
        await this.page.waitForTimeout(300);
        await input.press("Enter");
    }

    /**
     * Abandon the dropdown currently open. On the segment renderer this
     * leaves a half-built row behind on purpose — {@link selectKey} recycles
     * it.
     */
    async dismiss(): Promise<void> {
        await this.page.keyboard.press("Escape");
    }
}
