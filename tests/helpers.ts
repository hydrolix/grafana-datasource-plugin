// @ts-nocheck
import {BrowserContext, Locator, Page, test} from "@playwright/test";
import {DataSourceConfigPage, expect, PanelEditPage,} from "@grafana/plugin-e2e";
import allLabels from "../src/labels";
import {CreateDataSourcePageArgs} from "@grafana/plugin-e2e/dist/types";

/**
 * Install a passthrough route on `urlPattern` that captures the rawSql out of
 * every POST body and returns the array (mutated as requests come in).
 *
 * The fetch+fulfill pair is wrapped in try/catch: when the page navigates or
 * the test ends between the two calls, the Response is disposed and fulfill
 * throws `route.fulfill: Fetch response has been disposed`. Swallowing it
 * doesn't lose captured data (we push before fetching) and prevents the route
 * handler from failing the test (the failure looks like a flake on 11.x).
 */
export async function captureSqls(
    context: BrowserContext,
    urlPattern: string | RegExp = "**/api/ds/query**",
): Promise<string[]> {
    const sqls: string[] = [];
    await context.route(urlPattern, async (route, request) => {
        if (request.method() === "POST") {
            try {
                const json = JSON.parse(request.postData() ?? "");
                const sql = json?.queries?.[0]?.rawSql;
                if (sql) sqls.push(sql);
            } catch {
                // ignore non-JSON
            }
        }
        try {
            const response = await route.fetch();
            await route.fulfill({response});
        } catch {
            // route disposed / target closed: ignore
        }
    });
    return sqls;
}

/**
 * Like {@link captureSqls} but captures the full POST body as a string. Useful
 * when assertions need to read structure beyond `rawSql` (querySettings,
 * datasource ref, etc.).
 */
export async function captureRequestBodies(
    context: BrowserContext,
    urlPattern: string | RegExp = "**/api/ds/query**",
): Promise<string[]> {
    const bodies: string[] = [];
    await context.route(urlPattern, async (route, request) => {
        if (request.method() === "POST") {
            const body = request.postData() ?? "";
            if (body) bodies.push(body);
        }
        try {
            const response = await route.fetch();
            await route.fulfill({response});
        } catch {
            // route disposed / target closed: ignore
        }
    });
    return bodies;
}

/**
 * Capture paired request bodies + response bodies for every POST to
 * `urlPattern`. The returned `requests` and `responses` arrays are kept in
 * lockstep — index `i` in one corresponds to index `i` in the other — so
 * callers can locate a request by its payload (e.g. `queries[0].source ===
 * "annotation"`) and read the backend's reply at the same index.
 *
 * The response body is consumed once (`response.text()`) and replayed via
 * `fulfill({response, body})` so the page receives identical bytes. As with
 * {@link captureRequestBodies}, the fetch+fulfill pair is wrapped in try/catch
 * to swallow `route.fulfill: Fetch response has been disposed` when navigation
 * tears the route down mid-flight.
 */
export async function captureRequestsAndResponses(
    context: BrowserContext,
    urlPattern: string | RegExp = "**/api/ds/query**",
): Promise<{ requests: string[]; responses: string[] }> {
    const requests: string[] = [];
    const responses: string[] = [];
    await context.route(urlPattern, async (route, request) => {
        if (request.method() !== "POST") {
            try {
                const response = await route.fetch();
                await route.fulfill({response});
            } catch {
                // ignore
            }
            return;
        }
        const reqBody = request.postData() ?? "";
        try {
            const response = await route.fetch();
            try {
                const respText = await response.text();
                requests.push(reqBody);
                responses.push(respText);
                await route.fulfill({response, body: respText});
            } catch {
                try {
                    await route.fulfill({response});
                } catch {
                    // ignore
                }
            }
        } catch {
            // route disposed / target closed: ignore
        }
    });
    return {requests, responses};
}

/**
 * One captured/tagged `/api/ds/query` preload request, as recorded by
 * {@link captureAndTagPreloads}.
 */
export interface CapturedPreload {
    /** Fresh UUID tagged onto this request's `rawSql`. */
    uuid: string;
    /** The original (untagged) POST body, as sent by the browser. */
    body: string;
    /** The tagged `rawSql` actually forwarded to the backend. */
    mutatedSql: string;
}

/**
 * Route on `urlPattern` that captures every `/api/ds/query` POST body *and*
 * rewrites `queries[0].rawSql` to append a fresh-per-request UUID as a
 * trailing SQL comment (`-- e2e:<uuid>`) before forwarding. The mutated
 * request — not the original — is the one actually sent, so the UUID rides
 * through the backend's rounding + macro-expansion pipeline into the
 * executed statement, where {@link findQueryLogEntry} can find it in
 * `system.query_log` by UUID alone (immune to parallel-suite collisions,
 * unlike matching by table name or event time).
 *
 * `-- e2e:<uuid>` was verified end-to-end against the dev stack
 * (2026-08-25): the `/interpolate` resource echoes it back unmodified after
 * macro expansion (so the Go backend's AST/macro pipeline tolerates it), and
 * ClickHouse executes + logs the tagged statement unchanged. No `/* e2e:uuid
 * *\/` fallback was needed, but keep the option in mind if a future backend
 * change ever chokes on trailing `--` comments.
 *
 * Requests without a `queries[0].rawSql` (or non-JSON bodies) are forwarded
 * untouched and not recorded. As with {@link captureRequestBodies}, the
 * fetch+fulfill pair is wrapped in try/catch to swallow `Fetch response has
 * been disposed` when navigation tears the route down mid-flight.
 */
export async function captureAndTagPreloads(
    context: BrowserContext,
    urlPattern: string | RegExp = "**/api/ds/query**",
): Promise<CapturedPreload[]> {
    const captured: CapturedPreload[] = [];
    await context.route(urlPattern, async (route, request) => {
        if (request.method() !== "POST") {
            try {
                const response = await route.fetch();
                await route.fulfill({response});
            } catch {
                // route disposed / target closed: ignore
            }
            return;
        }
        const body = request.postData() ?? "";
        let postData = body;
        try {
            const json = JSON.parse(body);
            const sql = json?.queries?.[0]?.rawSql;
            if (sql) {
                const uuid = crypto.randomUUID();
                const mutatedSql = `${sql}\n-- e2e:${uuid}`;
                json.queries[0].rawSql = mutatedSql;
                postData = JSON.stringify(json);
                captured.push({uuid, body, mutatedSql});
            }
        } catch {
            // non-JSON / unparsable body: forward untouched, don't record
        }
        try {
            const response = await route.fetch({postData});
            await route.fulfill({response});
        } catch {
            // route disposed / target closed: ignore
        }
    });
    return captured;
}

/**
 * Strips a `-- e2e:<uuid>` (or `/* e2e:<uuid> *\/`) tag previously appended
 * by {@link captureAndTagPreloads} from an executed query's text, so two
 * queries that differ only by their UUID tag can be compared for equality.
 */
export function stripUuidComment(sql: string, uuid: string): string {
    return sql
        .replace(`-- e2e:${uuid}`, "")
        .replace(`/* e2e:${uuid} */`, "")
        .trimEnd();
}

/**
 * Looks up the executed (rounded, macro-expanded) query text in ClickHouse's
 * `system.query_log` by a UUID previously tagged via
 * {@link captureAndTagPreloads}. `system.query_log` is buffered, so this
 * issues `SYSTEM FLUSH LOGS` first and then polls a few times (the flush
 * response returning doesn't guarantee the row is queryable yet).
 *
 * Runs from the playwright container (or the host, when run there), so a
 * plain `fetch` against ClickHouse's HTTP interface is enough — no browser
 * context required.
 *
 * The host comes from `E2E_CLICKHOUSE_HTTP_HOST`, *not* from
 * `CLICKHOUSE_HOSTNAME`: the latter is the address Grafana uses to reach
 * ClickHouse (see {@link ConfigPageSteps.fillTestHttpDatasource}), which is
 * resolved inside the grafana container. This lookup is resolved by the
 * process running the tests, and the two differ whenever the suite runs
 * outside the compose network — GitHub Actions runs playwright on the runner
 * host, where the compose service name `clickhouse-server` does not resolve
 * and only the published `localhost:8123` does.
 */
export async function findQueryLogEntry(
    uuid: string,
    opts: {
        host?: string;
        port?: number;
        username?: string;
        password?: string;
        retries?: number;
        delayMs?: number;
    } = {},
): Promise<string> {
    const host =
        opts.host ??
        process.env.E2E_CLICKHOUSE_HTTP_HOST ??
        process.env.CLICKHOUSE_HOSTNAME ??
        "clickhouse-server";
    const port = opts.port ?? 8123;
    const username = opts.username ?? process.env.CLICKHOUSE_USERNAME ?? "testuser";
    const password = opts.password ?? process.env.CLICKHOUSE_PASSWORD ?? "testpass";
    const retries = opts.retries ?? 10;
    const delayMs = opts.delayMs ?? 500;
    const url = `http://${host}:${port}/`;
    const auth = "Basic " + Buffer.from(`${username}:${password}`).toString("base64");

    const post = async (sql: string): Promise<string> => {
        const res = await fetch(url, {
            method: "POST",
            headers: {Authorization: auth},
            body: sql,
        });
        if (!res.ok) {
            throw new Error(`ClickHouse HTTP query failed (${res.status}): ${await res.text()}`);
        }
        return res.text();
    };

    // Remembered so the final throw can tell "ClickHouse was unreachable"
    // apart from "the row genuinely never landed" — an unreachable host
    // (wrong hostname, container down) otherwise looks exactly like a
    // missing log entry.
    let lastError: unknown;

    try {
        await post("SYSTEM FLUSH LOGS");
    } catch (e) {
        // best-effort — poll below regardless, the row may already be flushed
        lastError = e;
    }

    const lookupSql = `SELECT query FROM system.query_log WHERE type = 'QueryFinish' AND query LIKE '%${uuid}%' AND query NOT LIKE '%query_log%' FORMAT JSON`;
    for (let attempt = 0; attempt < retries; attempt++) {
        try {
            const text = await post(lookupSql);
            const parsed = JSON.parse(text);
            if (parsed.data?.length) {
                return parsed.data[0].query as string;
            }
            lastError = undefined;
        } catch (e) {
            // transient — retry
            lastError = e;
        }
        await new Promise((resolve) => setTimeout(resolve, delayMs));
    }
    // node's fetch reports connection problems as a bare "TypeError: fetch
    // failed" and hides the real reason (ENOTFOUND, ECONNREFUSED…) in `cause`.
    const describe = (e: unknown) =>
        e instanceof Error && e.cause ? `${e} (${e.cause})` : String(e);

    throw new Error(
        `No system.query_log entry found for uuid ${uuid} after ${retries} retries` +
        ` (queried ${url})` +
        (lastError ? `; last error: ${describe(lastError)}` : "")
    );
}

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
            .getByRole("button", {name: "reset"})
            .or(loc.locator("button"))
            .first();
    }

    switch(loc: Locator): Locator {
        return loc.getByRole("switch").or(loc.getByLabel("Toggle switch")).first();
    }

    expandable(loc: Locator): Locator {
        return loc.getByRole("button", {name: `Expand section ${this.label}`});
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
 * Closes the "What's new in Grafana" features dialog if it is currently shown.
 * Grafana renders the dialog asynchronously after the page loads, so we wait
 * briefly for it to appear and silently move on if it never does.
 */
export const closeWhatsNewDialog = async (page: Page): Promise<void> => {
    const closeButton = page.locator(
        'xpath=//div[@role="dialog" and @aria-label="What\'s new in Grafana"]//button[@aria-label="Close"]'
    );
    try {
        await closeButton.waitFor({state: "visible", timeout: 3000});
        await closeButton.click({force: true});
        await closeButton.waitFor({state: "hidden", timeout: 3000});
    } catch {
        // Dialog wasn't shown within the timeout — nothing to close.
    }
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
 * Expands the "Query options" collapsible section in the panel editor query tab.
 * In Grafana 13 the "Query options" label is hidden and the toggle is a button
 * with aria-label "Expand query row" that flips to "Collapse query row" when open.
 */
export const expandQueryOptions = async (
    page: PanelEditPage
): Promise<void> => {
    try {
        const expandButton = page.ctx.page.getByRole("button", {
            name: /expand query row/i,
        });
        await expandButton.waitFor({state: "visible", timeout: 3000});
        await expandButton.click();
    } catch {
        // Dialog wasn't shown within the timeout — nothing to close.
        // try code for <= Grafana 12
        const queryOptionsButton = page.ctx.page.getByText(/Query options/i);
        await queryOptionsButton.click();
    }
};

/**
 * Sets the "Max data points" value in the panel editor.
 * Expands the Query options section and fills the input.
 */
export const setMaxDataPoints = async (
    page: PanelEditPage,
    value: number
): Promise<void> => {
    await expandQueryOptions(page);
    await page.ctx.page
        .locator(
            'xpath=//label[contains(text(), "Max data points")]/../div[1]//input'
        )
        .fill(value.toString());
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

    adHocConditionVariable(): Locator;

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
        const dsConfigPage = await createDataSourceConfigPage({
            type: "hydrolix-hydrolix-datasource",
            name: name,
            deleteDataSourceAfterTest: false,
        });
        await closeWhatsNewDialog(dsConfigPage.ctx.page);
        return dsConfigPage;
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
        const dsConfigPage = await createDataSourceConfigPage({
            type: "hydrolix-hydrolix-datasource",
            name: name,
            deleteDataSourceAfterTest: false,
        });
        await closeWhatsNewDialog(dsConfigPage.ctx.page);
        return dsConfigPage;
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
            .uncheck({force: true});
        await this.configPageLocator.secureSwitch().uncheck({force: true});

        // native connection
        await this.configPageLocator.protocolItem("native").check({force: true});
        await this.configPageLocator.port().fill("9000");
        await this.configPageLocator.username().fill(username);
        if (await this.configPageLocator.passwordReset().isVisible()) {
            await this.configPageLocator.passwordReset().click({force: true});
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
            .uncheck({force: true});
        await this.configPageLocator.secureSwitch().uncheck({force: true});

        // native connection
        await this.configPageLocator.protocolItem("http").check({force: true});
        await this.configPageLocator.port().fill("8123");
        await this.configPageLocator.username().fill(username);
        if (await this.configPageLocator.passwordReset().isVisible()) {
            await this.configPageLocator.passwordReset().click({force: true});
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
