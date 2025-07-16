// @ts-nocheck
import { Locator, Page, test } from "@playwright/test";
import {
  DataSourceConfigPage,
  expect,
  PanelEditPage,
} from "@grafana/plugin-e2e";
import allLabels from "../src/labels";
import { CreateDataSourcePageArgs } from "@grafana/plugin-e2e/dist/types";

/**
 * Decorator for Playwright steps
 */
export function step(target: Function, context: ClassMethodDecoratorContext) {
  return function replacementMethod(...args: any) {
    const name = this.constructor.name + "." + (context.name as string);
    return test.step(name, async () => {
      return await target.call(this, ...args);
    });
  };
}

/**
 * Wrapper which helps building simple locator chain for UI elements.
 */
class ElementContext {
  public testId: string;
  public label: string;

  constructor(name: string, labels: any) {
    this.testId = labels.testId;
    this.label = labels.label;
  }

  locator(loc: Locator): Locator {
    return loc.getByTestId(this.testId);
  }

  input(loc: Locator): Locator {
    return loc.getByRole("textbox").or(loc.locator("input")).first();
  }

  timerange(loc: Locator): Locator {
    return loc.getByRole("time").or(loc.locator("button")).first();
  }

  button(loc: Locator): Locator {
    return loc.getByRole("button").or(loc.locator("button")).first();
  }

  reset(loc: Locator): Locator {
    return loc
      .getByRole("button", { name: "reset" })
      .or(loc.locator("button"))
      .first();
  }

  switch(loc: Locator): Locator {
    return loc.getByRole("switch").or(loc.getByLabel("Toggle switch")).first();
  }

  expandable(loc: Locator): Locator {
    return loc.getByRole("button", { name: `Expand section ${this.label}` });
  }

  item(loc: Locator, name: string): Locator {
    return loc.getByLabel(name);
  }

  alert(loc: Locator): Locator {
    return loc.getByText(`${this.label} required`);
    // return loc.getByRole("alert", {name: this.label})
  }
}

/**
 * Page interface proxy handler.
 * It builds locator chain by the name of the interface method.
 * @param allLabels
 */
export const pageHandler = (allLabels: any): ProxyHandler<Page> => {
  return {
    get(target: Page, propKey) {
      return (...args) => {
        let propName = propKey.toString();
        let chains = propName.match(/[A-Za-z][^A-Z]*/g);
        if (!chains || chains.length === 0) {
          throw new SyntaxError(`Wrong method call ${propName}`);
        }

        let elemName = "";
        let i;
        for (i = 0; i < chains.length; i++) {
          const chain = chains[i];
          if (
            [
              "Input",
              "Item",
              "Switch",
              "Expandable",
              "Alert",
              "Timeselect",
              "Reset",
              "Button",
            ].indexOf(chain) >= 0
          ) {
            break;
          }

          elemName += chain;
        }
        const labelsKey = elemName as keyof typeof allLabels;
        let ctx = new ElementContext(elemName, allLabels[labelsKey]);
        const rootLoc = target.getByTestId("data-testid hydrolix_config_page");
        let loc: Locator = ctx.locator(rootLoc);
        do {
          const chain = chains.length === i ? "" : chains[i];
          switch (chain) {
            case "Item":
              loc = ctx.item(loc, args[0] as string);
              break;
            case "Switch":
              loc = ctx.switch(loc);
              break;
            case "Timeselect":
              loc = ctx.timerange(loc);
              break;
            case "Expandable":
              loc = ctx.expandable(rootLoc);
              break;
            case "Alert":
              loc = ctx.alert(rootLoc);
              break;
            case "Button":
              loc = ctx.button(loc);
              break;
            case "Reset":
              loc = ctx.reset(loc);
              break;
            case "Input":
            default:
              loc = ctx.input(loc);
          }
          i++;
        } while (i < chains.length);
        return loc;
      };
    },
  };
};

/**
 * Sets text into Monaco SQLQueryEditor
 *
 * @param refId Query refid (e.g. A, B, C, ...)
 * @param query text to set into the query editor
 * @param page PanelEditPage
 */
export const queryTextSet = async (
  refId: string,
  query: string,
  page: PanelEditPage
): Promise<void> => {
  const queryRow = page.getQueryEditorRow(refId);
  const editor = queryRow.getByRole("code").nth(0);
  await editor.click();
  await page.ctx.page.keyboard.press("ControlOrMeta+KeyA");
  await page.ctx.page.keyboard.type(query);
};

/**
 * Locators Interface for Hydrolix Datasource Configuration Page
 */
interface ConfigPageLocator {
  host(): Locator;

  hostAlert(): Locator;

  port(): Locator;

  portAlert(): Locator;

  useDefaultPortSwitch(): Locator;

  path(): Locator;

  username(): Locator;

  usernameAlert(): Locator;

  password(): Locator;

  passwordReset(): Locator;

  passwordAlert(): Locator;

  protocol(): Locator;

  protocolItem(name: string): Locator;

  secureSwitch(): Locator;

  skipTlsVerifySwitch(): Locator;

  defaultDatabase(): Locator;

  defaultRound(): Locator;

  adHocTableVariable(): Locator;

  adHocDefaultTimeRangeTimeselect(): Locator;

  dialTimeout(): Locator;

  queryTimeout(): Locator;

  additionalSettingsExpandable(): Locator;
}

/**
 * Hydrolix Configuration Page's Steps.
 */
export class ConfigPageSteps {
  readonly configPageLocator;

  constructor(readonly page: Page) {
    this.configPageLocator = ConfigPageSteps.getLocator(page);
  }

  /**
   * Factory for Interface's Locator Proxy
   * @param page
   */
  static getLocator(page: Page): ConfigPageLocator {
    return new Proxy(
      page as any,
      pageHandler(allLabels.components.config.editor)
    ) as ConfigPageLocator;
  }

  /**
   * Step to create Hydrolix Datasource Page
   * @param name name of the datasource in Grafana
   * @param createDataSourceConfigPage creation playwright handler
   */
  @step
  static async createDatasourceConfigPage(
    name: string,
    createDataSourceConfigPage: (
      args: CreateDataSourcePageArgs
    ) => Promise<DataSourceConfigPage>
  ) {
    return createDataSourceConfigPage({
      type: "hydrolix-hydrolix-datasource",
      name: name,
      deleteDataSourceAfterTest: false,
    });
  }

  /**
   * Step to create Hydrolix Datasource Page
   * @param name name of the datasource in Grafana
   * @param createDataSourceConfigPage creation playwright handler
   */
  @step
  async createDatasourceConfigPage(
    name: string,
    createDataSourceConfigPage: (
      args: CreateDataSourcePageArgs
    ) => Promise<DataSourceConfigPage>
  ) {
    return createDataSourceConfigPage({
      type: "hydrolix-hydrolix-datasource",
      name: name,
      deleteDataSourceAfterTest: false,
    });
  }

  /**
   * Fills out native datasource page for e2e tests.
   */
  @step
  async fillTestNativeDatasource() {
    const host = process.env.CLICKHOUSE_HOSTNAME ?? "clickhouse-server";
    const username = process.env.CLICKHOUSE_USERNAME ?? "testuser";
    const password = process.env.CLICKHOUSE_PASSWORD ?? "testpass";

    await this.configPageLocator.host().fill(host);

    await this.configPageLocator
      .useDefaultPortSwitch()
      .uncheck({ force: true });
    await this.configPageLocator.secureSwitch().uncheck({ force: true });

    // native connection
    await this.configPageLocator.protocolItem("native").check({ force: true });
    await this.configPageLocator.port().fill("9000");
    await this.configPageLocator.username().fill(username);
    if (await this.configPageLocator.passwordReset().isVisible()) {
      await this.configPageLocator.passwordReset().click({ force: true });
    }
    await this.configPageLocator.password().fill(password);
  }

  /**
   * Fills out http datasource page for e2e tests.
   */
  @step
  async fillTestHttpDatasource() {
    const host = process.env.CLICKHOUSE_HOSTNAME ?? "clickhouse-server";
    const username = process.env.CLICKHOUSE_USERNAME ?? "testuser";
    const password = process.env.CLICKHOUSE_PASSWORD ?? "testpass";

    await this.configPageLocator.host().fill(host);

    await this.configPageLocator
      .useDefaultPortSwitch()
      .uncheck({ force: true });
    await this.configPageLocator.secureSwitch().uncheck({ force: true });

    // native connection
    await this.configPageLocator.protocolItem("http").check({ force: true });
    await this.configPageLocator.port().fill("8123");
    await this.configPageLocator.username().fill(username);
    if (await this.configPageLocator.passwordReset().isVisible()) {
      await this.configPageLocator.passwordReset().click({ force: true });
    }
    await this.configPageLocator.password().fill(password);
  }

  /**
   * Save datasource page and verifies success
   * @param dsConfigPage playwright datasource page
   */
  @step
  async saveSuccess(dsConfigPage: DataSourceConfigPage) {
    await expect(dsConfigPage.saveAndTest()).toBeOK();
    await expect(dsConfigPage).toHaveAlert("success");
  }

  /**
   * Save datasource page and verifies error condition
   * @param error expected error
   * @param dsConfigPage datasource playwright page
   */
  @step
  async saveError(error: string, dsConfigPage: DataSourceConfigPage) {
    await expect(dsConfigPage.saveAndTest()).not.toBeOK();
    await expect(dsConfigPage).toHaveAlert("error", {
      hasText: error,
    });
  }
}
