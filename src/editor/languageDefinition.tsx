import {
    LanguageDefinition,
    StatementPlacementProvider,
    SuggestionKindProvider,
    TableIdentifier,
} from "@grafana/plugin-ui";
import {MACROS} from "./macros";
import {Monaco} from "@grafana/ui";
import {SQLMonarchLanguage} from "@grafana/plugin-ui/dist/src/components/SQLEditor/standardSql/types";
import {MetadataProvider} from "./metadataProvider";
import {SQLCompletionItemProvider} from "@grafana/plugin-ui/dist/src/components/SQLEditor/types";
import {format} from 'sql-formatter';
import {OPERATORS} from "./operators";
import {FUNCTIONS} from "./functions";
import {Props} from "../components/QueryEditor";


export const languageDefinition: (metadataProvider: MetadataProvider, props: Props)
    => LanguageDefinition = (metadataProvider: MetadataProvider, props: Props) => {
    return {
        id: 'sql',
        completionProvider: (m: Monaco, language: SQLMonarchLanguage) => {
            const completionProvider = {
                schemas: {
                    resolve: () => metadataProvider.schemas()
                },
                tables: {
                    resolve: (t: TableIdentifier) => metadataProvider.tables(t)
                },
                columns: {
                    resolve: (t: TableIdentifier) => metadataProvider.columns(t)
                },
                triggerCharacters: ['.', ' ', ',', '(', "'"],
                supportedFunctions: () => FUNCTIONS,

                customSuggestionKinds: customSuggestionKinds(),
                customStatementPlacement,
                supportedMacros: () => MACROS,
                supportedOperators: () => OPERATORS

            } as SQLCompletionItemProvider;
            setKeywords(metadataProvider, m, language);

            setHotKeys(m, props)

            return completionProvider;
        },
        formatter: s => format(s, {language: 'mysql'})
            .replace(/(\$__\w*\s\()/g, (m) => m.replace(/\s/g, ''))
            .replace("SETTINGS", "\nSETTINGS\n ")
    };
}

enum CustomStatementPosition {
    AfterQuery = 'afterQuery',
}

const customStatementPlacement: StatementPlacementProvider = () => [
    {
        id: CustomStatementPosition.AfterQuery,
        resolve: (currentToken, previousKeyword) => {
            return Boolean(
                !currentToken?.getNextNonWhiteSpaceToken() &&
                (
                    !currentToken?.isKeyword() && previousKeyword?.value.toUpperCase() === 'BY' ||
                    !currentToken?.isKeyword() && previousKeyword?.value.toUpperCase() === 'TIME' ||
                    !currentToken?.isKeyword() && previousKeyword?.value.toUpperCase() === 'ASC' ||
                    !currentToken?.isKeyword() && previousKeyword?.value.toUpperCase() === 'DESC' ||
                    (!currentToken?.isKeyword() && previousKeyword?.value.toUpperCase() === 'FROM' &&
                        !currentToken?.getPreviousNonWhiteSpaceToken()?.isKeyword()) ||
                    (!currentToken?.isKeyword() && previousKeyword?.value.toUpperCase() === 'LIMIT' &&
                        !currentToken?.getPreviousNonWhiteSpaceToken()?.isKeyword()) ||
                    (!currentToken?.isKeyword() && previousKeyword?.value.toUpperCase() === 'WHERE'
                        && !currentToken?.getPreviousNonWhiteSpaceToken()?.isKeyword()
                        && !currentToken?.isOperator()
                        && !currentToken?.getPreviousNonWhiteSpaceToken()?.isOperator()))
            );
        },
    }
];

const setKeywords = (metadataProvider: MetadataProvider, m: Monaco, language: SQLMonarchLanguage) => {

    m.languages.getLanguages().map(l => l.id).filter(l => l.startsWith('sql-')).forEach(languageId => {

        m.languages.setMonarchTokensProvider(languageId!, {
            ...language,
            keywords: (language.keywords?.length ? language.keywords : []).concat(['SETTINGS']),
        })
    });

}
const setHotKeys = (m: Monaco, props: Props) => {
    m.editor.addEditorAction({
        id: "executeCurrentAndAdvance",
        label: "Execute Block and Advance",
        keybindings: [m.KeyMod.CtrlCmd | m.KeyCode.Enter, m.KeyMod.WinCtrl | m.KeyCode.Enter],
        run: props.onRunQuery,
    });
};

export const customSuggestionKinds: () => SuggestionKindProvider = () => () => [
    {
        id: 'SETTINGS',
        suggestionsResolver: () => Promise.resolve([{label: "SETTINGS"}]),
        applyTo: [CustomStatementPosition.AfterQuery],

    }
];

export const overrideSqlStringColor = (m: Monaco) => {
    m.editor.getEditors()
        // @ts-ignore
        .map(e => e?._themeService?._theme as IStandaloneThemeData)
        .forEach(currentTheme => {
            m.editor.defineTheme('hdx-default', {
                base: currentTheme.base,
                inherit: true,
                colors: currentTheme.colors,
                rules: [
                    {token: 'string.sql', foreground: 'CE9178'},
                ]

            });
            m.editor.setTheme('hdx-default');
        })
}
