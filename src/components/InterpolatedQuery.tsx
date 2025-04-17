import React from "react";
import { IconButton } from "@grafana/ui";

export function InterpolatedQuery({
  sql,
  error,
  showSQL,
}: {
  sql: string;
  error: string;
  showSQL: boolean;
}) {
  return (
    showSQL && (
      <>
        <h4 style={{ margin: "10px 0px 5px 0px" }}>Interpolated Query</h4>
        {error ? (
          <pre
            style={{
              position: "relative",
              minHeight: 40,
              color: "red",
            }}
          >
            {error}
          </pre>
        ) : (
          <pre
            style={{
              position: "relative",
              minHeight: 40,
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
              name="copy"
              size="lg"
              tooltip="copy to clipboard"
              onClick={() => navigator.clipboard.writeText(sql)}
            />
          </pre>
        )}
      </>
    )
  );
}
