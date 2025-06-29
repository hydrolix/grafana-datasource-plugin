import {
  LanguageDefinition,
  StatementPlacementProvider,
  SuggestionKindProvider,
  TableIdentifier,
} from "@grafana/plugin-ui";
import { MACROS } from "./macros";
import { Monaco } from "@grafana/ui";
import { SQLMonarchLanguage } from "@grafana/plugin-ui/dist/src/components/SQLEditor/standardSql/types";
import { SQLCompletionItemProvider } from "@grafana/plugin-ui/dist/src/components/SQLEditor/types";
import { format } from "sql-formatter";
import { OPERATORS } from "./operators";
import { FUNCTIONS } from "./functions";
import { Props } from "../components/QueryEditor";
import { applyHotKey, updateOptions } from "./editorUtils";

export const languageDefinition: (
  props: Props,
  setMonaco: (value: Monaco | null) => void
) => LanguageDefinition = (
  props: Props,
  setMonaco: (value: Monaco | null) => void
) => {
  return {
    id: "sql",
    completionProvider: (m: Monaco, language: SQLMonarchLanguage) => {
      const completionProvider = {
        schemas: {
          resolve: () => props.datasource.metadataProvider.schemas(),
        },
        tables: {
          resolve: (t: TableIdentifier) =>
            props.datasource.metadataProvider.tables(t),
        },
        columns: {
          resolve: (t: TableIdentifier) =>
            props.datasource.metadataProvider.columns(t),
        },
        triggerCharacters: [".", " ", ",", "(", "'"],
        supportedFunctions: () => FUNCTIONS,

        customSuggestionKinds: customSuggestionKinds(),
        customStatementPlacement,
        supportedMacros: () => MACROS,
        supportedOperators: () => OPERATORS,
      } as SQLCompletionItemProvider;
      updateOptions(m);
      setKeywords(m, language);
      applyHotKey(m, props);
      setMonaco(m);

      return completionProvider;
    },
    formatter: (s) =>
      format(s, { language: "mysql" })
        .replace(/(\$__\w*\s\()/g, (m) => m.replace(/\s/g, ""))
        .replace("SETTINGS", "\nSETTINGS\n "),
  };
};

enum CustomStatementPosition {
  AfterQuery = "afterQuery",
}

const customStatementPlacement: StatementPlacementProvider = () => [
  {
    id: CustomStatementPosition.AfterQuery,
    resolve: (currentToken, previousKeyword) => {
      return Boolean(
        !currentToken?.getNextNonWhiteSpaceToken() &&
          ((!currentToken?.isKeyword() &&
            previousKeyword?.value.toUpperCase() === "BY") ||
            (!currentToken?.isKeyword() &&
              previousKeyword?.value.toUpperCase() === "TIME") ||
            (!currentToken?.isKeyword() &&
              previousKeyword?.value.toUpperCase() === "ASC") ||
            (!currentToken?.isKeyword() &&
              previousKeyword?.value.toUpperCase() === "DESC") ||
            (!currentToken?.isKeyword() &&
              previousKeyword?.value.toUpperCase() === "FROM" &&
              !currentToken?.getPreviousNonWhiteSpaceToken()?.isKeyword()) ||
            (!currentToken?.isKeyword() &&
              previousKeyword?.value.toUpperCase() === "LIMIT" &&
              !currentToken?.getPreviousNonWhiteSpaceToken()?.isKeyword()) ||
            (!currentToken?.isKeyword() &&
              previousKeyword?.value.toUpperCase() === "WHERE" &&
              !currentToken?.getPreviousNonWhiteSpaceToken()?.isKeyword() &&
              !currentToken?.isOperator() &&
              !currentToken?.getPreviousNonWhiteSpaceToken()?.isOperator()))
      );
    },
  },
];

const setKeywords = (m: Monaco, language: SQLMonarchLanguage) => {
  m.languages
    .getLanguages()
    .map((l) => l.id)
    .filter((l) => l.startsWith("sql-"))
    .forEach((languageId) => {
      m.languages.setMonarchTokensProvider(languageId!, {
        ...language,
        keywords: (language.keywords?.length ? language.keywords : []).concat([
          "SETTINGS",
        ]),
      });
    });
};

export const customSuggestionKinds: () => SuggestionKindProvider = () => () =>
  [
    {
      id: "SETTINGS",
      suggestionsResolver: () => Promise.resolve([{ label: "SETTINGS" }]),
      applyTo: [CustomStatementPosition.AfterQuery],
    },
  ];
