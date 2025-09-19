import { GrafanaTheme2 } from "@grafana/data";
import { css } from "@emotion/css";
import { Button, TextArea, useStyles2 } from "@grafana/ui";
import React, {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from "react";

function calculateLineHeightInPx(e: HTMLElement) {
  const style = window.getComputedStyle(e);
  const fontSize = parseFloat(style.fontSize);
  const rawLineHeight = style.lineHeight;

  if (rawLineHeight === "normal") {
    return fontSize * 1.2;
  }

  if (rawLineHeight.endsWith("px")) {
    return parseFloat(rawLineHeight);
  }

  if (!isNaN(Number(rawLineHeight))) {
    return fontSize * parseFloat(rawLineHeight);
  }

  return fontSize * 1.2;
}

export interface AssistantInputProps {
  onSend: (message: string) => void;
  onInterrupt: () => void;
  thinking: boolean;
  minRows?: number;
  maxRows?: number;
}

export const AssistantInput = forwardRef<
  HTMLTextAreaElement,
  AssistantInputProps
>(function AiAssistantInput(
  {
    onSend,
    onInterrupt,
    thinking,
    minRows = 2,
    maxRows = 6,
  }: AssistantInputProps,
  ref
) {
  const inputRef = useRef<HTMLTextAreaElement>(null);
  useImperativeHandle(ref, () => inputRef.current!);

  const [input, setInput] = useState("");
  const styles = useStyles2(getStyles);

  const resizeInput = useCallback(() => {
    const e = inputRef.current;
    if (e) {
      e.rows = minRows;
      const currentRows = Math.floor(
        e.scrollHeight / calculateLineHeightInPx(e)
      );
      e.rows = Math.min(maxRows, currentRows);
    }
  }, [minRows, maxRows]);

  useEffect(() => {
    resizeInput();
  }, [input, resizeInput]);

  const handleSendOrInterrupt = () => {
    if (thinking) {
      onInterrupt();
    } else {
      const message = input.trim();
      if (message) {
        onSend(message);
        setInput("");
      }
    }
  };

  return (
    <div className={styles.container}>
      <TextArea
        autoFocus
        ref={inputRef}
        className={styles.input}
        cols={40}
        rows={2}
        value={input}
        placeholder="Ask about your query in the editor"
        onChange={(e) => setInput(e.currentTarget.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && (e.shiftKey || e.metaKey)) {
            handleSendOrInterrupt();
          }
        }}
      />
      <div className={styles.footer}>
        <Button
          className={styles.button}
          variant={thinking ? "secondary" : "primary"}
          onMouseDown={(e) => {
            e.preventDefault();
            handleSendOrInterrupt();
          }}
        >
          {!thinking ? "Send" : "Interrupt"}
        </Button>
      </div>
    </div>
  );
});

const getStyles = (theme: GrafanaTheme2) => ({
  input: css({
    flex: 1,
    width: "100%",
    padding: 0,
    border: "none",
    fontSize: theme.typography.size.md,
    resize: "none",
    overflowY: "auto",
    lineHeight: theme.typography.body.lineHeight,
    minHeight: `${theme.spacing(6)}`,
    boxSizing: "border-box",
    background: "none",
    userSelect: "none",

    outline: "none",
    boxShadow: "none",
    transition: "none",

    "&:focus": {
      outline: "none",
      boxShadow: "none",
      background: "none",
    },

    "&:hover": {
      border: "none",
      background: "none",
    },

    "&:active": {
      border: "none",
      background: "none",
    },
  }),

  button: css({
    alignSelf: "flex-end" as const,
  }),

  footer: css({
    display: "inline-flex",
    alignItems: "center",
    justifyContent: "flex-end",
    verticalAlign: "middle",
    lineHeight: 0,
    marginTop: theme.spacing(1.5),
  }),

  container: css({
    display: "flex",
    alignItems: "stretch",
    flexDirection: "column" as const,
    color: theme.components.input.text,
    background: theme.components.input.background,
    border: `1px solid ${theme.components.input.borderColor}`,
    borderRadius: theme.shape.radius.default,
    padding: theme.spacing(1.5, 1.5),
  }),
});
