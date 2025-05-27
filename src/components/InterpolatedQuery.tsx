import React from "react";
import { IconButton, IconName, PopoverContent } from "@grafana/ui";

export function InterpolatedQuery({
  sql,
  error,
  showSQL,
  dirty,
  showErrors,
}: {
  sql: string;
  error: string;
  showSQL: boolean;
  dirty: boolean;
  showErrors: boolean;
}) {
  let readyColor = error ? "#800202" : "#002f0d";
  let borderColor = dirty ? "#421701" : readyColor;
  let icon: IconName = dirty ? "spinner" : "copy";
  let iconTooltip: PopoverContent = dirty ? "processing" : "copy to clipboard";
  return (
    showSQL && (
      <>
        <h4 style={{ margin: "10px 0px 5px 0px" }}>Interpolated Query</h4>
        {(!sql || showErrors) && error ? (
          <pre
            style={{
              position: "relative",
              minHeight: 80,
              color: "red",
              borderColor: readyColor,
            }}
          >
            {showErrors ? error : ""}
          </pre>
        ) : (
          <pre
            style={{
              position: "relative",
              minHeight: 80,
              borderColor,
            }}
          >
            {sql}
            <IconButton
              style={{
                position: "absolute",
                top: "10px",
                right: "10px",
                display: "flex",
                alignItems: "center",
                gap: "10px",
              }}
              aria-label="copy-formatted-data-to-clipboard"
              name={icon}
              size="lg"
              tooltip={iconTooltip}
              onClick={() => navigator.clipboard.writeText(sql)}
            />
          </pre>
        )}
      </>
    )
  );
}
