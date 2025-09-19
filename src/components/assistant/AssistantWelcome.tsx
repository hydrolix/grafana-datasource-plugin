import React from "react";
import { Button, useStyles2 } from "@grafana/ui";
import { css } from "@emotion/css";
import { GrafanaTheme2 } from "@grafana/data";

export const AssistantQuestions = {
    explain: "What does this query do?",
    validate: "Is this query correct?",
    fix: "Why is this query failing?",
    optimize: "Can you optimize this query?",
};

const suggestions = [
  AssistantQuestions.explain,
  AssistantQuestions.fix,
  AssistantQuestions.validate,
  AssistantQuestions.optimize,
];

export interface AssistantWelcomeProps {
  onSend: (message: string) => void;
}

export function AssistantWelcome({ onSend }: AssistantWelcomeProps) {
  const styles = useStyles2(getStyles);

  return (
    <div className={styles.container}>
      <div className={styles.title}>Hydrolix Assistant</div>
      <div className={styles.subtitle}>
        Hydrolix Assistant helps you understand what queries do, fixes them when they break, and recommends improvements.
      </div>
      <div className={styles.cloud}>
        {suggestions.map((text, idx) => (
          <Button
            key={idx}
            variant="secondary"
            className={styles.suggestion}
            onMouseDown={(e) => {
              e.preventDefault();
              onSend(text);
            }}
          >
            {text}
          </Button>
        ))}
      </div>
    </div>
  );
}

const getStyles = (theme: GrafanaTheme2) => ({
  container: css({
    display: "flex",
    flexDirection: "column",
    alignItems: "center",
    justifyContent: "center",
    height: "100%",
    padding: theme.spacing(2),
    textAlign: "center",
    gap: theme.spacing(3),
  }),
  title: css({
    textAlign: "center",
    fontSize: theme.typography.h3.fontSize,
    fontWeight: theme.typography.fontWeightBold,
    margin: 0,
    lineHeight: theme.typography.h3.lineHeight,
  }),
  subtitle: css({
    textAlign: "center",
    color: theme.colors.text.secondary,
    fontSize: theme.typography.size.md,
    lineHeight: theme.typography.body.lineHeight,
    margin: 0,
  }),
  cloud: css({
    display: "flex",
    flexWrap: "wrap",
    justifyContent: "center",
    gap: theme.spacing(1.5),
  }),
  suggestion: css({
    whiteSpace: "nowrap",
  }),
});
