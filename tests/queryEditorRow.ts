import {Locator, Page} from "@playwright/test";
import {PanelEditPage} from "@grafana/plugin-e2e";
import {openGrafanaSelect, pickOptionByPrefix} from "./grafanaSelect";

/**
 * Page-object wrapping a single query editor row in a panel editor.
 *
 * Centralises locator chains for the row's controls (Monaco SQL editor,
 * round input, query type picker, Query Settings collapsible) so individual
 * tests don't re-discover them. Each method is version-tolerant via the
 * locator helpers in `./grafanaSelect`.
 */
export class QueryEditorRow {
    readonly row: Locator;
    private readonly page: Page;
    private readonly panelEditPage: PanelEditPage;

    constructor(panelEditPage: PanelEditPage, refId: string = "A") {
        this.panelEditPage = panelEditPage;
        this.row = panelEditPage.getQueryEditorRow(refId);
        this.page = panelEditPage.ctx.page;
    }

    /**
     * Type SQL into the Monaco editor. Selects all existing content first so
     * repeated calls overwrite rather than append.
     */
    async setSql(sql: string): Promise<void> {
        const editor = this.row.getByRole("code").nth(0);
        await editor.click();
        await this.page.keyboard.press("ControlOrMeta+KeyA");
        await this.page.keyboard.type(sql);
    }

    /**
     * Fill the row's round-duration input. Empty string clears it.
     */
    async setRound(duration: string): Promise<void> {
        await this.row.getByTestId("data-testid round input").fill(duration);
    }

    /**
     * Expand the Query Settings collapsible. No-op if already open (Grafana
     * toggles on click; callers should call once per test).
     */
    async openQuerySettings(): Promise<void> {
        await this.row.getByText("Query Settings", {exact: true}).click();
    }

    /**
     * Add a Query Setting row, pick `name` from the Select, and fill `value`.
     * Assumes {@link openQuerySettings} has already been called (the "new
     * setting" button is hidden until then).
     */
    async addQuerySetting(name: string, value: string): Promise<void> {
        await this.row.getByLabel("new setting").click();
        await openGrafanaSelect(this.row);
        await pickOptionByPrefix(this.page, name);
        await this.row.getByLabel(name).fill(value);
    }

    /**
     * Click the Show/Hide Interpolated Query button. Pass `true` to reveal
     * the panel and `false` to hide it.
     */
    async toggleInterpolatedQuery(show: boolean): Promise<void> {
        const labelRegex = show
            ? /Show Interpolated Query/i
            : /Hide Interpolated Query/i;
        await this.page.getByRole("button", {name: labelRegex}).click();
    }
}
