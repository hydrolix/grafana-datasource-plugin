import { Icon, Monaco, Spinner, useTheme2 } from "@grafana/ui";
import React, { useCallback, useMemo, useState } from "react";
import { css } from "@emotion/css";
import { InterpolationResult } from "../types";
import { removeUnderline, underline } from "../editor/editorUtils";
import "./ValidationBar.css";

interface Props {
  monaco: Monaco | null;
  interpolationResult: InterpolationResult | null;
  query: string;
}

export function ValidationBar({ monaco, query, interpolationResult }: Props) {
  let [validating, setValidating] = useState<boolean>(false);

  const underlineError = useCallback(
    (line: number, start: number, end: number) => {
      underline(monaco, line, start, end);
    },
    [monaco]
  );
  const removeUnderlineError = useCallback(() => {
    removeUnderline(monaco);
  }, [monaco]);

  useMemo(() => {
    if (interpolationResult?.hasError) {
      const fullMessage = interpolationResult.error ?? "";
      const errorRegExp = /^line\s(\d*):(\d*)/;

      const [message, _, arrows] = fullMessage.split("\n");
      const match = errorRegExp.exec(message);
      if (match) {
        const line = parseInt(match[1], 10) + 1;
        const start = parseInt(match[2], 10) + 1;
        const end = start + arrows.trim().length + 1;
        underlineError(line, start, end);
      } else {
        removeUnderlineError();
      }
    }
  }, [
    interpolationResult?.error,
    interpolationResult?.hasError,
    removeUnderlineError,
    underlineError,
  ]);

  useMemo(() => {
    setValidating(interpolationResult?.originalSql !== query);
  }, [query, interpolationResult?.originalSql]);

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

  return (
    <div className={styles.container}>
      {interpolationResult?.interpolatedSql && (
        <>
          {validating && (
            <div className={styles.info}>
              <Spinner inline={true} size={12} /> Validating query...
            </div>
          )}
          {!validating &&
            !interpolationResult.hasError &&
            !interpolationResult.hasWarning && (
              <div className={styles.valid}>
                <Icon name="check" /> No errors found
              </div>
            )}
          {!validating && interpolationResult.hasError && (
            <div className={styles.error}>
              <Icon name="exclamation-circle" /> {interpolationResult.error}
            </div>
          )}
          {!validating &&
            !interpolationResult.hasError &&
            interpolationResult.hasWarning && (
              <div className={styles.warning}>
                <Icon name="exclamation-triangle" />{" "}
                {interpolationResult.warning}
              </div>
            )}
        </>
      )}
    </div>
  );
}
