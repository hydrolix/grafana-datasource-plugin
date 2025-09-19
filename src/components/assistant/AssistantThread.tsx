import { GrafanaTheme2 } from "@grafana/data";
import { Alert, CustomScrollbar, Icon, useTheme2 } from "@grafana/ui";
import { AssistantMessage } from "../../assistant/assistantConversation";
import { css, cx } from "@emotion/css";
import React, { useCallback, useEffect, useRef, useState } from "react";
import { AssistantThreadMessage } from "./AssistantThreadMessage";
import { useThrottleCallback } from "@react-hook/throttle";

export interface AssistantThreadProps {
  className?: string;
  messages: AssistantMessage[];
  onApplyQuery: (query: string) => void;
  onReset: () => void;
  errorMessage?: string;
}

export const AssistantThread: React.FC<AssistantThreadProps> = ({
  className,
  messages,
  errorMessage,
  onApplyQuery,
  onReset,
}) => {
  const theme = useTheme2();
  const padding = (theme.components as any).drawer?.padding ?? 2;
  const fadeHeight = theme.spacing.gridSize * padding * 3;

  const styles = getStyles(fadeHeight, theme);

  const [scrollRef, setScrollRef] = useState<HTMLDivElement | null>(null);
  const scrollRefCallback = useCallback((node: HTMLDivElement) => {
    setScrollRef(node);
  }, []);

  const [showScrollButton, setShowScrollButton] = useState(false);
  const onScroll = useThrottleCallback(() => {
    if (!scrollRef) {
      return;
    }
    setShowScrollButton(
      scrollRef.scrollTop + scrollRef.clientHeight <=
        scrollRef.scrollHeight - fadeHeight
    );
  }, 60);

  const mountedRef = useRef(false);
  const lastMessageRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!mountedRef.current) {
      mountedRef.current = true;
      onScroll();
      return;
    }
    if (lastMessageRef.current) {
      lastMessageRef.current.scrollIntoView({
        block: "start",
        behavior: "smooth",
      });
    }
  }, [onScroll, messages]);

  const onScrollToBottom = () => {
    if (!scrollRef) {
      return;
    }
    scrollRef.scrollTo({ top: scrollRef.scrollHeight, behavior: "smooth" });
  };

  return (
    <div className={styles.threadContainer}>
      <CustomScrollbar
        autoHeightMin="0"
        autoHeightMax="100%"
        scrollRefCallback={scrollRefCallback}
        onScroll={onScroll}
      >
        <div className={styles.scrollable}>
          <div className={cx(styles.messagesContainer, className)}>
            {messages.map((m, i) => (
              <div
                key={m.id}
                ref={i === messages.length - 1 ? lastMessageRef : null}
              >
                <AssistantThreadMessage
                  messageId={m.id}
                  onApplyQuery={onApplyQuery}
                />
              </div>
            ))}
          </div>
          {errorMessage && (
            <Alert
              title={""}
              severity={"error"}
              buttonContent={"Reset"}
              onRemove={onReset}
            >
              {errorMessage}
            </Alert>
          )}
          <div className={styles.fade} />
        </div>
      </CustomScrollbar>
      <button
        aria-label="Scroll to bottom"
        className={cx(
          styles.scrollButtonDefault,
          showScrollButton && styles.scrollButtonVisible
        )}
        onMouseDown={(e) => {
          e.preventDefault();
          onScrollToBottom();
        }}
      >
        <Icon name="arrow-down" size="lg" />
      </button>
    </div>
  );
};

const getStyles = (fadeHeight: number, theme: GrafanaTheme2) => {
  return {
    threadContainer: css({
      position: "relative",
      overflow: "hidden",
      height: "100%",
      width: "100%",
    }),

    messagesContainer: css({
      display: "flex",
      flexDirection: "column",
      justifyContent: "flex-end",
      width: "100%",
    }),

    scrollable: css({
      position: "relative",
      display: "block",
      minHeight: 0,
      height: "100%",
      paddingTop: theme.spacing(4),
    }),

    fade: css({
      position: "sticky",
      bottom: 0,
      left: 0,
      right: 0,
      height: `${fadeHeight}px`,
      pointerEvents: "none",
      background: `linear-gradient(transparent, ${theme.colors.background.primary})`,
      zIndex: 1,
    }),

    scrollButtonDefault: css({
      position: "absolute",
      left: "50%",
      bottom: theme.spacing(2),
      right: theme.spacing(2),
      width: theme.spacing(4),
      height: theme.spacing(4),
      borderRadius: "50%",
      border: "none",
      background: theme.colors.emphasize(
        theme.colors.background.secondary,
        0.05
      ),
      boxShadow: theme.shadows.z1,
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      opacity: 0,
      transform: "translateX(-50%) scale(0.8)",
      transition: "opacity 0.25s ease, transform 0.25s ease",
      pointerEvents: "none",
      zIndex: 2,
    }),

    scrollButtonVisible: css({
      opacity: 1,
      transform: "translateX(-50%) scale(1)",
      pointerEvents: "auto",
    }),
  };
};
