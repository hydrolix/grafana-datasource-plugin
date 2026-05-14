import {Page} from "@playwright/test";
import {pickOption} from "./grafanaSelect";

/**
 * Page-object for a dashboard's on-page template-variable picker.
 *
 * The picker DOM differs by Grafana version:
 *   - 10.x: `<button aria-label="$variableName">` containing the value.
 *   - 11.x–13.x: react-select wrapped in `[data-value=""]` (no role on the
 *     clickable surface).
 * The `.or()` chain resolves to whichever matches on the current Grafana.
 *
 * Once open, option items also differ (role="option" vs role="checkbox") —
 * delegated to {@link pickOption}.
 */
export class VariablePicker {
    constructor(
        private readonly name: string,
        private readonly page: Page,
    ) {}

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
        await this.page
            .getByRole("button", {name: this.name, exact: true})
            .or(this.page.locator('[data-value=""]'))
            .first()
            .click();
    }
}
