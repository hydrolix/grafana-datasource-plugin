import React from "react";
import {Button, useTheme2} from "@grafana/ui";
import {Prism as SyntaxHighlighter} from "react-syntax-highlighter";
import {
    vs,
    vscDarkPlus,
} from "react-syntax-highlighter/dist/esm/styles/prism";
import {GrafanaTheme2} from "@grafana/data";
import {css} from "@emotion/css";

export interface AssistantCodeProps {
    language?: string;
    onApply?: (query: string) => void;
    children: string;
}

export const AssistantCode: React.FC<AssistantCodeProps> = ({
                                                                language,
                                                                onApply,
                                                                children,
                                                            }) => {
    const theme = useTheme2();
    const styles = getStyles(theme);
    const style = theme.isLight ? vs : vscDarkPlus;
    const canApply = language === "sql";
    return (
        <div className={styles.container}>
            <div className={styles.header}>
                <div className={styles.language}>{language || "unknown"}</div>
                <div style={{width: "95%"}}></div>
                <Button
                    size="sm"
                    className={styles.button}
                    style={{gap: "3em"}}
                    variant="secondary"
                    tooltip="Copy"
                    icon={"copy"}
                    onClick={() => {
                        if (!navigator.clipboard) {
                            throw new Error("Browser don't have support for native clipboard.");
                        }
                        navigator.clipboard.writeText(children);
                    }}
                ></Button>
                {canApply && (
                    <Button
                        size="sm"
                        className={styles.button}
                        variant="secondary"
                        tooltip="Apply this query to the editor"
                        onClick={() => {
                            onApply && onApply(children);
                        }}
                    >
                        {"Apply"}
                    </Button>
                )}
            </div>
            <SyntaxHighlighter
                language={language}
                showInlineLineNumbers={false}
                wrapLines={true}
                style={style}
                codeTagProps={{
                    style: {whiteSpace: "pre-wrap", wordBreak: "break-word"},
                }}
                customStyle={{
                    backgroundColor: "transparent",
                    margin: 0,
                    padding: 0,
                    border: "none",
                }}
            >
                {children}
            </SyntaxHighlighter>
        </div>
    );
};

const getStyles = (theme: GrafanaTheme2) => {
    const height = theme.components.height.sm;
    return {
        container: css({
            display: "flex",
            alignItems: "stretch",
            flexDirection: "column" as const,
            color: theme.components.input.text,
            backgroundColor: theme.colors.background.canvas,
            border: `1px solid ${theme.components.input.borderColor}`,
            borderRadius: theme.shape.radius.default,
            padding: theme.spacing(1.5, 1.5),
        }),

        header: css({
            display: "inline-flex",
            alignItems: "center",
            justifyContent: "space-between",
            verticalAlign: "middle",
            width: "100%",

            "&&": {
                marginBottom: theme.spacing(1),
            },
        }),

        language: css({
            display: "block",
            background: theme.colors.emphasize(theme.colors.background.canvas, 0.025),
            padding: theme.spacing(0, 1),
            borderRadius: theme.shape.radius.default,
            fontSize: theme.typography.size.sm,
            height: theme.spacing(height),
            lineHeight: `${theme.spacing.gridSize * height}px`,
            verticalAlign: "middle",
            textTransform: "lowercase",
            color: theme.colors.text.secondary,
        }),

        button: css({
            margin: 0,
        }),
    };
};
