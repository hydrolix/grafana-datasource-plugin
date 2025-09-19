import { GrafanaTheme2 } from "@grafana/data";
import { css } from "@emotion/css";
import { Drawer, Icon, IconButton, useStyles2 } from "@grafana/ui";
import React, { useEffect, useRef } from "react";
import { AssistantInput } from "./AssistantInput";
import { AssistantThread } from "./AssistantThread";
import {
  useAssistantConversation,
  useAssistantConversationState,
} from "./AssistantContext";
import { AssistantWelcome } from "./AssistantWelcome";

export interface AssistantPanelProps {
  query: string;
  onApplyQuery: (query: string) => void;
  onClose: () => void;
}

export function AssistantPanel({
  query,
  onApplyQuery,
  onClose,
}: AssistantPanelProps) {
  const assistant = useAssistantConversation();
  const messages = useAssistantConversationState((s) => s.messages);
  const errorMessage = useAssistantConversationState((s) => s.errorMessage);
  const thinking = useAssistantConversationState((s) => s.thinking);

  const styles = useStyles2(getStyles);

  const inputRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    requestAnimationFrame(() => {
      inputRef.current?.focus();
    });
  }, []);

  const onSend = (message: string, includeQuery = false) =>
    assistant.addMessage(message, query);

  const onReset = async () => {
    await assistant.reset();
  };

  const showWelcomePage = messages?.length === 0;

  return (
    <Drawer
      size="md"
      scrollableContent={false}
      onClose={onClose}
      title={
        <div className={styles.titleContainer}>
          <div className={styles.title}>
            <Icon name={"ai"} />
            <span>Hydrolix Assistant</span>
          </div>
          <div className={styles.actions}>
            <IconButton
              className={styles.action}
              name="trash-alt"
              variant="secondary"
              tooltip="Reset"
              onClick={onReset}
            />
            <IconButton
              className={styles.action}
              name="times"
              variant="secondary"
              tooltip="Close"
              onClick={onClose}
            />
          </div>
        </div>
      }
    >
      <div className={styles.contentContainer}>
        {showWelcomePage ? (
          <AssistantWelcome
            onSend={(message: string) => onSend(message, true)}
          />
        ) : (
          <AssistantThread
            className={styles.thread}
            messages={messages}
            errorMessage={errorMessage}
            onReset={onReset}
            onApplyQuery={onApplyQuery}
          />
        )}
        <AssistantInput
          ref={inputRef}
          onSend={onSend}
          onInterrupt={() => assistant.abort()}
          thinking={thinking}
        />
        <span className={styles.note}>Always review accuracy of responses</span>
      </div>
    </Drawer>
  );
}

const getStyles = (theme: GrafanaTheme2) => {
  const padding = (theme.components as any).drawer?.padding ?? 2;
  return {
    titleContainer: css({
      display: "flex",
      alignItems: "center",
      justifyContent: "space-between",
      width: "100%",
      padding: theme.spacing(padding, padding, 0, padding + 1.5),
    }),

    title: css({
      fontSize: theme.typography.size.md,
      fontWeight: theme.typography.fontWeightRegular,
      color: theme.colors.text.primary,
      display: "inline-flex",
      alignItems: "center",
      gap: theme.spacing(1),
    }),

    actions: css({}),

    action: css({
      height: "24px",
      width: "24px",
    }),

    contentContainer: css({
      display: "flex",
      flexDirection: "column",
      height: "100%",
      minHeight: 0,
    }),

    thread: css({
      padding: theme.spacing(0, 1.5),
    }),

    note: css({
      width: "100%",
      marginTop: theme.spacing(1.5),
      textAlign: "center",
      color: theme.colors.text.secondary,
    }),
  };
};
