import { Icon, Spinner, useTheme2 } from "@grafana/ui";
import React, { useMemo } from "react";
import { css } from "@emotion/css";
import { ValidationResult, ValidationState } from "../types";

export function toValidationState(result: ValidationResult): ValidationState {
  if (result.skipped) {
    return { status: "idle" };
  }
  if (result.error) {
    return { status: "error", message: result.error };
  }
  if (result.warning) {
    return { status: "warning", message: result.warning };
  }
  return { status: "valid" };
}

interface Props {
  state: ValidationState;
}

export function ValidationBar({ state }: Props) {
  const theme = useTheme2();
  const styles = useMemo(() => {
    return {
      // Rendered even when idle, so the layout doesn't jump.
      container: css`
        border: 1px solid ${theme.colors.border.medium};
        border-top: none;
        padding: ${theme.spacing(0.5, 0.5, 0.5, 0.5)};
        min-height: 28px;
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
        white-space: pre-wrap;
        word-break: break-word;
      `,
      valid: css`
        color: ${theme.colors.success.text};
      `,
      info: css`
        color: ${theme.colors.text.secondary};
      `,
    };
  }, [theme]);

  return (
    <div className={styles.container} data-testid="query-validation-bar">
      {state.status === "validating" && (
        <div className={styles.info}>
          <Spinner inline={true} size={12} /> Validating query...
        </div>
      )}
      {state.status === "valid" && (
        <div className={styles.valid}>
          <Icon name="check" /> Query is valid
        </div>
      )}
      {state.status === "error" && (
        <div className={styles.error}>
          <Icon name="exclamation-circle" /> {state.message}
        </div>
      )}
      {state.status === "warning" && (
        <div className={styles.warning}>
          <Icon name="exclamation-triangle" /> {state.message}
        </div>
      )}
    </div>
  );
}
