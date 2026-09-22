import {Page} from "@playwright/test";
import {pickOption} from "./grafanaSelect";

/**
 * Page-object for a dashboard's on-page template-variable picker.
 *
 * Across every supported Grafana the picker is a react-select wrapped in
 * `[data-value=""]` with no role on the clickable surface, so that wrapper is
 * the only reliable handle. It carries no variable name, hence `.first()`:
 * this page-object addresses the first picker on the page and is only correct
 * on a dashboard carrying a single template variable. Scope by the submenu
 * label first if a test ever needs a dashboard with two.
 */
export class VariablePicker {
    constructor(private readonly page: Page) {}

    /**
     * Open the picker and select the option whose accessible name equals
     * `value`. For multi-select / regex-named variables, use `pickOption`
     * directly after calling {@link open}.
     */
    async select(value: string): Promise<void> {
        await this.open();
        await pickOption(this.page, value);
    }

    /**
     * Open the picker without selecting anything. Use when the caller needs
     * to perform a more elaborate interaction with the menu.
     */
    async open(): Promise<void> {
        await this.page.locator('[data-value=""]').first().click();
    }
}
