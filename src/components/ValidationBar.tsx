import { Icon, Monaco, Spinner, useTheme2 } from "@grafana/ui";
import React, { useMemo, useState } from "react";
import { css } from "@emotion/css";
import { InterpolationResult } from "../types";

interface Props {
  monaco: Monaco | null;
  interpolationResult: InterpolationResult | null;
  query: string;
}

export function ValidationBar({ monaco, query, interpolationResult }: Props) {
  let [validating, setValidating] = useState<boolean>(false);

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
