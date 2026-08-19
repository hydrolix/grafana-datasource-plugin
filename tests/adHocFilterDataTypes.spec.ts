import { test, expect } from "@grafana/plugin-e2e";
// @ts-ignore
import {
  captureRequestsAndResponses,
  closeWhatsNewDialog,
  ConfigPageSteps,
} from "./helpers";
import { DashboardBuilder } from "./dashboardBuilder";

/**
 * Runtime ad-hoc filtering over UUID / IPv4 / IPv6 columns.
 *
 * Adding UUID/IPv4/IPv6 to SUPPORTED_TYPES made those columns selectable as
 * ad-hoc filter keys. That is only half the feature: the value the user picks
 * then has to survive the whole round trip —
 *
 *   Grafana filter state
 *     -> applyTemplateVariables (src/datasource.ts) attaches `filters`
 *     -> POST /api/ds/query
 *     -> AdHocFilterMacro (pkg/plugin/macros_adhoc.go) resolves the table from
 *        the AST and checks the key against the DESCRIBE key set
 *     -> buildFilterCondition emits SQL
 *     -> ClickHouse has to accept a *string literal* compared against a
 *        UUID / IPv4 / IPv6 column
 *
 * Nothing above the converter layer was covered before: the eligibility list is
 * unit-tested (src/editor/metadataProvider.test.ts), the emitted SQL is
 * unit-tested (pkg/plugin/macros_adhoc_test.go), and the literal comparison was
 * only ever checked by hand against a container. These tests close that gap.
 *
 * Assertions read the /api/ds/query response frames rather than panel DOM.
 * The panel here is an aggregate, so there is exactly one row to check, and
 * reading the frame avoids the table-grid column virtualization that forced
 * tests/dataTypes.spec.ts into two-column queries.
 *
 * A silently dropped filter is the failure mode worth naming: if the macro
 * cannot match the key it skips the filter and returns "1=1", so the query
 * succeeds and returns *every* row. Every single-row case below would then
 * report cnt=2, which is why each case asserts an exact count rather than
 * merely "some rows came back".
 */

test.describe.configure({ mode: "serial" });

const ADHOC_VAR = "hdx_adhoc";
const TABLE_VAR = "hdx_table";

/** Both rows of e2e.datatypes, keyed by the UUID that identifies each. */
const ROW1_UUID = "61f0c404-5cb3-11e7-907b-a6006ad3dba0";
const ROW2_UUID = "9d3b1f3a-0c2e-4c9a-9a8b-1c2d3e4f5a6b";

/**
 * count() plus a row fingerprint. min(uuid_col) identifies *which* row matched,
 * not just how many: a filter that accidentally selected the wrong row would
 * still report cnt=1. min() over a UUID column is binary-ordered, and
 * ROW1_UUID < ROW2_UUID, so a two-row match fingerprints as ROW1_UUID.
 *
 * Projecting min(uuid_col) also runs the UUID converter over an *aggregate*
 * result column, whose DatabaseTypeName is still "UUID".
 */
const PANEL_SQL =
  "select count() as cnt, min(uuid_col) as row_uuid " +
  "from e2e.datatypes where $__adHocFilter()";

interface FilterCase {
  title: string;
  key: string;
  operator: string;
  value?: string;
  values?: string[];
  expectedCount: number;
  /** Omitted when expectedCount is 0 — min() over no rows is the zero UUID. */
  expectedRowUuid?: string;
}

const cases: FilterCase[] = [
  {
    title: "UUID column, '=' against a canonical UUID literal",
    key: "uuid_col",
    operator: "=",
    value: ROW1_UUID,
    expectedCount: 1,
    expectedRowUuid: ROW1_UUID,
  },
  {
    title: "IPv4 column, '=' against a dotted-quad literal",
    key: "v4_col",
    operator: "=",
    value: "1.2.3.4",
    expectedCount: 1,
    expectedRowUuid: ROW1_UUID,
  },
  {
    title: "IPv6 column, '=' against an IPv6 literal",
    key: "v6_col",
    operator: "=",
    value: "2001:db8::1",
    expectedCount: 1,
    expectedRowUuid: ROW1_UUID,
  },
  {
    // The Hydrolix-shaped case, and the panel-to-filter round trip. v6_mapped is
    // IPv6 holding ::ffff:1.2.3.4, which the plugin renders padded — matching
    // ClickHouse's own toString() (design decision D5). So this literal is
    // exactly the text a user copies out of a panel cell.
    title:
      "IPv4-mapped IPv6 column, '=' against the padded form the panel displays",
    key: "v6_mapped",
    operator: "=",
    value: "::ffff:1.2.3.4",
    expectedCount: 1,
    expectedRowUuid: ROW1_UUID,
  },
  {
    // ClickHouse parses a filter literal into an IPv6 value before comparing, so
    // a dotted-quad literal still matches a mapped address even though it is no
    // longer what the panel shows. Kept because it is the form a user typing
    // from memory would reach for.
    title:
      "IPv4-mapped IPv6 column, '=' against a dotted-quad literal is still accepted",
    key: "v6_mapped",
    operator: "=",
    value: "1.2.3.4",
    expectedCount: 1,
    expectedRowUuid: ROW1_UUID,
  },
  {
    title: "IPv4 column, '!=' excludes the matching row",
    key: "v4_col",
    operator: "!=",
    value: "1.2.3.4",
    expectedCount: 1,
    expectedRowUuid: ROW2_UUID,
  },
  {
    // Multi-value operator -> `key IN ('…','…')`. plugin.json sets
    // multiValueFilterOperators, so Grafana offers "=|" for these keys.
    title: "IPv4 column, '=|' with two literals matches both rows",
    key: "v4_col",
    operator: "=|",
    values: ["1.2.3.4", "5.6.7.8"],
    expectedCount: 2,
    expectedRowUuid: ROW1_UUID,
  },
  {
    // The __null__ sentinel getTagValues offers for a nullable column. UUID/IP
    // types are not "string" to buildFilterCondition's isString check, so this
    // must take the plain `IS NULL` branch — the string branch would also emit
    // `= '__null__'`, which no UUID column can parse.
    title: "Nullable(UUID) column, '=' __null__ selects the NULL row",
    key: "uuid_null",
    operator: "=",
    value: "__null__",
    expectedCount: 1,
    expectedRowUuid: ROW2_UUID,
  },
  {
    title: "Nullable(UUID) column, '!=' __null__ selects the populated row",
    key: "uuid_null",
    operator: "!=",
    value: "__null__",
    expectedCount: 1,
    expectedRowUuid: ROW1_UUID,
  },
  {
    title: "Nullable(IPv6) column, '=' __null__ selects the NULL row",
    key: "v6_null",
    operator: "=",
    value: "__null__",
    expectedCount: 1,
    expectedRowUuid: ROW2_UUID,
  },
  {
    // '=~' routes through `toString(key) LIKE '…'` — ClickHouse's own formatter
    // rather than a parsed comparison. Because the plugin now renders IPv6
    // columns the same way toString() does, the value copied from a panel cell
    // works under '=~' as well as under '=' (the pair of cases above and this
    // one are the whole point of D5: one text form, every operator).
    title:
      "IPv4-mapped IPv6 column, '=~' matches the same padded form the panel displays",
    key: "v6_mapped",
    operator: "=~",
    value: "::ffff:1.2.3.4",
    expectedCount: 1,
    expectedRowUuid: ROW1_UUID,
  },
  {
    // The boundary that keeps the two operator families honest: '=~' compares
    // rendered text, so a dotted-quad literal cannot match a column ClickHouse
    // renders padded (the LIKE pattern carries no wildcards). '=' accepts it
    // because it parses first. Asserting 0 pins that distinction — if '=~' ever
    // started parsing IP literals, this test would catch the change.
    title:
      "IPv4-mapped IPv6 column, '=~' does not match a dotted-quad literal (text comparison)",
    key: "v6_mapped",
    operator: "=~",
    value: "1.2.3.4",
    expectedCount: 0,
  },
];

/**
 * Pulls the first frame for the panel query out of a captured /api/ds/query
 * response. Skips the metadata queries the ad-hoc machinery issues alongside
 * it by requiring the panel's own SQL on the request side.
 */
function findPanelFrame(
  requests: string[],
  responses: string[],
): any | undefined {
  for (let i = requests.length - 1; i >= 0; i--) {
    let req: any;
    try {
      req = JSON.parse(requests[i]);
    } catch {
      continue;
    }
    const query = (req?.queries ?? []).find(
      (q: any) =>
        typeof q?.rawSql === "string" && q.rawSql.includes("$__adHocFilter()"),
    );
    if (!query) continue;

    let resp: any;
    try {
      resp = JSON.parse(responses[i]);
    } catch {
      continue;
    }
    const frames = resp?.results?.[query.refId]?.frames ?? [];
    if (frames.length) {
      return { frame: frames[0], query };
    }
  }
  return undefined;
}

/** Column values of a Grafana JSON data frame, by field name. */
function columnByName(frame: any, name: string): any[] | undefined {
  const index = (frame?.schema?.fields ?? []).findIndex(
    (f: any) => f.name === name,
  );
  return index < 0 ? undefined : frame?.data?.values?.[index];
}

let datasource: { type: string; uid: string };

test.beforeEach(async ({ createDataSourceConfigPage, page }) => {
  if (datasource !== undefined) {
    return;
  }
  const steps = new ConfigPageSteps(page);
  const dsConfigPage = await steps.createDatasourceConfigPage(
    "adhoc datatypes",
    createDataSourceConfigPage,
  );
  await steps.fillTestNativeDatasource();

  // adHocTableVariable names the dashboard variable holding the table whose
  // columns the key picker offers. The backend macro resolves the table from
  // the SQL AST independently, so filtering works without it — but production
  // datasources always set it, and so should the fixture.
  await steps.configPageLocator
    .additionalSettingsExpandable()
    .click({ force: true });
  await steps.configPageLocator.adHocTableVariable().fill(TABLE_VAR);

  await steps.saveSuccess(dsConfigPage);
  datasource = dsConfigPage.datasource;
});

cases.forEach((c) => {
  test(`ad-hoc filter: ${c.title}`, async ({
    gotoDashboardPage,
    page,
    context,
  }) => {
    const { uid } = await new DashboardBuilder(page, datasource)
      .withTitle(`adhoc-datatypes-${c.key}-${c.operator}-${Date.now()}`)
      .addCustomVariable({ name: TABLE_VAR, query: "e2e.datatypes" })
      .addAdHocVariable({
        name: ADHOC_VAR,
        filters: [
          {
            key: c.key,
            operator: c.operator,
            value: c.value,
            values: c.values,
          },
        ],
      })
      .addPanel({ title: "adhoc", rawSql: PANEL_SQL })
      .withTimeRange("2025-04-09T00:00:00.000Z", "2025-04-09T23:59:59.000Z")
      .create();

    const { requests, responses } = await captureRequestsAndResponses(context);

    const dashboardPage = await gotoDashboardPage({ uid });
    await closeWhatsNewDialog(page);
    await dashboardPage.refreshDashboard();

    await expect
      .poll(() => findPanelFrame(requests, responses) !== undefined, {
        timeout: 30000,
      })
      .toBe(true);

    const { frame, query } = findPanelFrame(requests, responses)!;

    // The filter reached the backend at all. Without this, a frontend-side
    // regression (filters never attached) would look identical to a backend
    // one that matched every row.
    expect(
      query.filters,
      "filters should ride along with the panel query",
    ).toEqual([expect.objectContaining({ key: c.key, operator: c.operator })]);

    // The query itself must not have errored — a literal ClickHouse cannot
    // parse for the column type surfaces here, not as a wrong row count.
    expect(
      frame?.schema,
      `panel query returned no schema for ${c.key}`,
    ).toBeTruthy();

    const counts = columnByName(frame, "cnt");
    expect(counts, "cnt column should be present").toBeTruthy();
    expect(
      Number(counts![0]),
      `filter ${c.key} ${c.operator} ${c.value ?? JSON.stringify(c.values)} ` +
        `should match ${c.expectedCount} row(s); cnt=2 means the macro dropped ` +
        `the filter and fell back to 1=1`,
    ).toBe(c.expectedCount);

    if (c.expectedRowUuid) {
      const uuids = columnByName(frame, "row_uuid");
      expect(uuids, "row_uuid column should be present").toBeTruthy();
      expect(uuids![0], "the wrong row matched").toBe(c.expectedRowUuid);
    }
  });
});
