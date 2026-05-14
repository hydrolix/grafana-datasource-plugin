import {Page, expect} from "@playwright/test";

interface DatasourceRef {
    type: string;
    uid: string;
}

interface PanelOpts {
    id?: number;
    title?: string;
    type?: "table" | "timeseries" | "stat" | string;
    rawSql: string;
    gridPos?: { x: number; y: number; w: number; h: number };
}

interface CustomVariableOpts {
    name: string;
    /** Comma-separated values list, as Grafana's "Custom" variable expects. */
    query: string;
    /** Selected value. Defaults to the first item in `query`. */
    current?: string;
}

interface CreateResult {
    uid: string;
}

/**
 * Fluent builder that POSTs a dashboard JSON model to Grafana's
 * `/api/dashboards/db` endpoint. Use this instead of driving the
 * Settings → Variables UI: that UI has reshuffled across Grafana 10/11/12/13
 * (button names, tab labels, type-picker, value inputs) and is the single
 * largest source of cross-version flake. The JSON model is stable.
 *
 * Each `add*` / `with*` call returns `this`, so the builder reads top-down:
 *
 *   const { uid } = await new DashboardBuilder(page, dsConfigPage.datasource)
 *     .withTitle("template-var-test")
 *     .addCustomVariable({ name: "tbl", query: "macros,no_such_table" })
 *     .addPanel({ rawSql: "SELECT … FROM e2e.$tbl …" })
 *     .withTimeRange("2025-04-10T00:00:00.000Z", "2025-04-10T23:59:59.000Z")
 *     .create();
 *   const dashboardPage = await gotoDashboardPage({ uid });
 */
export class DashboardBuilder {
    private title: string;
    private panels: unknown[] = [];
    private variables: unknown[] = [];
    private timeRange: { from: string; to: string } | undefined;
    private nextPanelId = 1;

    constructor(
        private readonly page: Page,
        private readonly datasource: DatasourceRef,
        title?: string,
    ) {
        this.title = title ?? `dashboard-${Date.now()}`;
    }

    withTitle(title: string): this {
        this.title = title;
        return this;
    }

    withTimeRange(from: string, to: string): this {
        this.timeRange = {from, to};
        return this;
    }

    addCustomVariable(opts: CustomVariableOpts): this {
        const values = opts.query.split(",").map((v) => v.trim());
        const current = opts.current ?? values[0];
        this.variables.push({
            name: opts.name,
            type: "custom",
            query: opts.query,
            current: {text: current, value: current, selected: true},
            options: values.map((v) => ({
                text: v,
                value: v,
                selected: v === current,
            })),
            includeAll: false,
            multi: false,
        });
        return this;
    }

    addPanel(opts: PanelOpts): this {
        const id = opts.id ?? this.nextPanelId++;
        this.panels.push({
            id,
            title: opts.title ?? `panel-${id}`,
            type: opts.type ?? "table",
            datasource: {type: this.datasource.type, uid: this.datasource.uid},
            targets: [
                {
                    refId: "A",
                    datasource: {
                        type: this.datasource.type,
                        uid: this.datasource.uid,
                    },
                    rawSql: opts.rawSql,
                },
            ],
            gridPos: opts.gridPos ?? {x: 0, y: (id - 1) * 8, w: 24, h: 8},
        });
        return this;
    }

    async create(): Promise<CreateResult> {
        const dashboard: Record<string, unknown> = {
            title: this.title,
            schemaVersion: 38,
            panels: this.panels,
            templating: {list: this.variables},
            timezone: "utc",
        };
        if (this.timeRange) {
            dashboard.time = {from: this.timeRange.from, to: this.timeRange.to};
        }
        const resp = await this.page.request.post("/api/dashboards/db", {
            data: {dashboard, overwrite: true},
        });
        expect(resp.ok(), `POST /api/dashboards/db failed: ${resp.status()}`).toBe(true);
        const body = await resp.json();
        return {uid: body.uid};
    }
}
