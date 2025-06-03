import {
  CustomVariableModel,
  DataSourceInstanceSettings,
  PluginType,
  ScopedVars,
} from "@grafana/data";
import { DataSource } from "../datasource";
import { AstResponse, HdxDataSourceOptions } from "../types";
import {
  DataSourceWithBackend,
  getBackendSrv,
  setBackendSrv,
  TemplateSrv,
} from "@grafana/runtime";
import { of } from "rxjs";

export const MockDataSourceInstanceSettings: DataSourceInstanceSettings<HdxDataSourceOptions> =
  {
    jsonData: {
      defaultDatabase: "sample",
      adHocTableVariable: "table",
      adHocTimeColumnVariable: "timefilter",
    },
    id: 0,
    uid: "",
    type: "",
    name: "Mock Hydrolix Data Source",
    meta: {
      id: "",
      name: "",
      type: PluginType.datasource,
      info: {
        author: {
          name: "",
        },
        description: "",
        links: [],
        logos: {
          large: "",
          small: "",
        },
        screenshots: [],
        updated: "",
        version: "",
      },
      module: "",
      baseUrl: "",
    },
    readOnly: false,
    access: "direct",
  };

const queryMock = jest.fn().mockReturnValue(of({ data: [] }));
jest
  .spyOn(DataSourceWithBackend.prototype, "query")
  .mockImplementation((args) => queryMock(args));
jest.spyOn(DataSource.prototype, "getAst").mockReturnValue(
  Promise.resolve({
    originalSql: "",
    error: false,
    data: {},
  } as AstResponse)
);

const separatorMap = new Map<string, string>([
  ["pipe", "|"],
  ["raw", ","],
  ["text", " + "],
]);

export function setupTemplateServiceMock(
  variables?: CustomVariableModel[]
): TemplateSrv {
  return {
    replace: jest
      .fn()
      .mockImplementation(
        (input: string, scopedVars?: ScopedVars, format?: string) => {
          if (!input) {
            return "";
          }
          let output = input;
          ["datasource", "dimension"].forEach((name) => {
            const variable = scopedVars ? scopedVars[name] : undefined;
            if (variable) {
              output = output.replace("$" + name, variable.value);
              output = output.replace(`$\{${name}}`, variable.value);
            }
          });

          if (variables) {
            variables.forEach((variable) => {
              let repVal = "";
              let value =
                format === "text"
                  ? variable.current.text
                  : variable.current.value;
              let separator = separatorMap.get(format ?? "raw");
              if (Array.isArray(value)) {
                repVal = value.join(separator);
              } else {
                repVal = value;
              }
              output = output.replace("$" + variable.name, repVal);
              output = output.replace(`\$\{${variable.name}}`, repVal);
              output = output.replace("[[" + variable.name + "]]", repVal);
            });
          }
          return output;
        }
      ),
    getVariables: jest.fn().mockReturnValue(variables ?? []),
    containsTemplate: jest.fn(),
    updateTimeRange: jest.fn(),
  };
}

export function setupDataSourceMock({
  variables,
  getMock = jest.fn(),
  customInstanceSettings = MockDataSourceInstanceSettings,
}: {
  variables?: CustomVariableModel[];
  getMock?: jest.Func;
  customInstanceSettings?: DataSourceInstanceSettings<HdxDataSourceOptions>;
}) {
  const templateService = setupTemplateServiceMock(variables);
  const datasource = new DataSource(customInstanceSettings, templateService);
  const fetchMock = jest.fn().mockReturnValue(of({}));
  setBackendSrv({
    ...getBackendSrv(),
    fetch: fetchMock,
    get: getMock,
  });
  return { datasource, fetchMock, queryMock, templateService };
}
