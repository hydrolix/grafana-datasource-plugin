import { setupDataSourceMock } from "../__mocks__/datasource";
import { languageDefinition } from "./languageDefinition";
import { Props } from "../components/QueryEditor";
import { Monaco } from "@grafana/ui";
import { SQLMonarchLanguage } from "@grafana/plugin-ui/dist/src/components/SQLEditor/standardSql/types";
import { FUNCTIONS } from "./functions";
import { MACROS } from "./macros";
import { OPERATORS } from "./operators";

describe("language definition", () => {
  const { datasource } = setupDataSourceMock({});
  const props = {
    datasource: datasource,
    onRunQuery: () => {},
  } as Props;
  beforeEach(() => {
    jest.clearAllMocks();
  });
  it("should format queries", () => {
    let ld = languageDefinition(props, (_) => {});
    let formated = ld.formatter!("SELECT 1");
    expect(formated).toBe("SELECT\n  1");
  });

  describe("completion provider", () => {
    beforeEach(() => {
      jest.clearAllMocks();
    });

    let SQL_LANGUAGE = {
      tokenizer: {},
    } as SQLMonarchLanguage;
    const MONACO_10 = {
      KeyMod: {},
      KeyCode: {},
      languages: {
        getLanguages: () => [],
      },
      editor: {
        _standaloneKeybindingService: {
          addDynamicKeybinding: () => {},
        },
        updateOptions: () => {},
      },
    } as unknown as Monaco;
    const MONACO_11 = {
      KeyMod: {},
      KeyCode: {},
      languages: {
        getLanguages: () => [],
      },
      editor: {
        getEditors: () => [
          {
            _standaloneKeybindingService: {
              addDynamicKeybinding: () => {},
            },
            updateOptions: () => {},
          },
        ],
      },
    } as unknown as Monaco;
    const completionItemProvider = languageDefinition(props, (_) => {})
      .completionProvider!(MONACO_10, SQL_LANGUAGE);
    it("should get functions", () => {
      let result = completionItemProvider.supportedFunctions!();
      expect(result).toBe(FUNCTIONS);
    });
    it("should get macros", () => {
      let result = completionItemProvider.supportedMacros!();
      expect(result).toBe(MACROS);
    });
    it("should get operators", () => {
      let result = completionItemProvider.supportedOperators!();
      expect(result).toBe(OPERATORS);
    });
    it("should get schemas", async () => {
      jest
        .spyOn(datasource.metadataProvider, "schemas")
        .mockReturnValue(Promise.resolve([{ name: "schema" }]));
      let result = await completionItemProvider.schemas!.resolve();
      expect(result).toEqual([{ name: "schema" }]);
    });
    it("should get tables", async () => {
      jest
        .spyOn(datasource.metadataProvider, "tables")
        .mockReturnValue(Promise.resolve([{ name: "table" }]));
      let result = await completionItemProvider.tables!.resolve({});
      expect(result).toEqual([{ name: "table" }]);
    });
    it("should get columns", async () => {
      jest
        .spyOn(datasource.metadataProvider, "columns")
        .mockReturnValue(Promise.resolve([{ name: "column" }]));
      let result = await completionItemProvider.columns!.resolve({});
      expect(result).toEqual([{ name: "column" }]);
    });
    it("should get customSuggestionKinds", async () => {
      let result = completionItemProvider.customSuggestionKinds!();
      expect(result.length).toBe(1);
    });
    it("should get customStatementPlacement", async () => {
      let result = completionItemProvider.customStatementPlacement!();
      expect(result.length).toBe(1);
    });
    it("should not fail on grafana 11", () => {
      const result = languageDefinition(props, (_) => {}).completionProvider!(
        MONACO_11,
        SQL_LANGUAGE
      );
      expect(result).not.toBeNull();
    });
  });
});
