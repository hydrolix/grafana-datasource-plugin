import { Icon, Monaco, Spinner, useTheme2 } from "@grafana/ui";
import React, { useCallback, useMemo, useState } from "react";
import { css } from "@emotion/css";
import { AstResponse, ValidationResult } from "../types";
import { removeUnderline, underline } from "../editor/editorUtils";
import { useDebounce } from "react-use";
import { DataSource } from "../datasource";
import "./ValidationBar.css";

interface Props {
  monaco: Monaco | null;
  datasource: DataSource;
  query: string;
}

export function ValidationBar({ monaco, datasource, query }: Props) {
  let [validationResult, setValidationResult] = useState<ValidationResult>({
    noQuery: true,
    validating: false,
    hasErrors: false,
    hasWarnings: false,
  });

  const underlineError = useCallback(
    (line: number, start: number, end: number) => {
      underline(monaco, line, start, end);
    },
    [monaco]
  );
  const removeUnderlineError = useCallback(() => {
    removeUnderline(monaco);
  }, [monaco]);

  const ast = useCallback(
    async (query: string) => {
      setValidationResult({
        validating: true,
        hasWarnings: false,
        hasErrors: false,
      });
      removeUnderlineError();
      let astResponse: AstResponse = await new Promise((resolve) =>
        setTimeout(
          () =>
            datasource
              .postResource("ast", {
                data: { query },
              })
              .then((a) => resolve(a as AstResponse)),
          1000
        )
      );
      if (astResponse.error) {
        const fullMessage = astResponse.error_message;
        console.log(fullMessage);
        const errorRegExp = /^line\s(\d*):(\d*)/;

        const [message, _, arrows] = fullMessage.split("\n");
        const match = errorRegExp.exec(message);
        if (match) {
          const line = parseInt(match[1], 10) + 1;
          const start = parseInt(match[2], 10) + 1;
          const end = start + arrows.trim().length + 1;
          underlineError(line, start, end);
          setValidationResult({
            validating: false,
            hasWarnings: false,
            hasErrors: true,
            error: message,
          });
        } else {
          setValidationResult({
            validating: false,
            hasWarnings: false,
            hasErrors: true,
            error: fullMessage,
          });
        }
      } else {
        setValidationResult({
          noQuery: !astResponse.data,
          validating: false,
          hasWarnings: false,
          hasErrors: false,
        });
        console.log("ast", astResponse.data);
      }
    },
    [datasource, underlineError, removeUnderlineError]
  );

  useDebounce(
    async () => {
      ast(query);
    },
    500,
    [query]
  );

  const theme = useTheme2();
  const styles = useMemo(() => {
    return {
      container: css`
        border: 1px solid ${theme.colors.border.medium};
        border-top: none;
        padding: ${theme.spacing(0.5, 0.5, 0.5, 0.5)};
        height: 28px;
        display: flex;
        flex-grow: 1;
        justify-content: space-between;
        font-size: ${theme.typography.bodySmall.fontSize};
      `,
      warning: css`
        color: ${theme.colors.warning.text};
        font-size: ${theme.typography.bodySmall.fontSize};
        font-family: ${theme.typography.fontFamilyMonospace};
      `,
      error: css`
        color: ${theme.colors.error.text};
        font-size: ${theme.typography.bodySmall.fontSize};
        font-family: ${theme.typography.fontFamilyMonospace};
      `,
      valid: css`
        color: ${theme.colors.success.text};
      `,
      info: css`
        color: ${theme.colors.text.secondary};
      `,
      hint: css`
        color: ${theme.colors.text.disabled};
        white-space: nowrap;
        cursor: help;
      `,
    };
  }, [theme]);
  let style = {};
  // if (!showTools && validationResult === undefined) {
  //   style = { height: 0, padding: 0, visibility: "hidden" };
  // }
  return (
    <div className={styles.container} style={style}>
      {!validationResult.noQuery && (
        <>
          {validationResult.validating && (
            <div className={styles.info}>
              <Spinner inline={true} size={12} /> Validating query...
            </div>
          )}
          {!validationResult.validating &&
            !validationResult.hasErrors &&
            !validationResult.hasWarnings && (
              <div className={styles.valid}>
                <Icon name="check" /> This query is valid
              </div>
            )}
          {!validationResult.validating && validationResult.hasErrors && (
            <div className={styles.error}>
              <Icon name="exclamation-circle" /> {validationResult.error}
            </div>
          )}
          {!validationResult.validating &&
            !validationResult.hasErrors &&
            validationResult.hasWarnings && (
              <div className={styles.warning}>
                <Icon name="exclamation-triangle" /> {validationResult.warning}
              </div>
            )}
        </>
      )}
    </div>
  );
}
