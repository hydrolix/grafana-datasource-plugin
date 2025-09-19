import React from "react";
import ReactMarkdown, { Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import { GrafanaTheme2 } from "@grafana/data";
import { css } from "@emotion/css";
import { Icon, Tooltip, useTheme2 } from "@grafana/ui";
import { AssistantCode } from "./AssistantCode";
import {
  useAssistantConversationState,
  useAssistantConversationStateActions,
} from "./AssistantContext";
import { AssistantThinkingProgress } from "./AssistantThinkingProgress";

export interface AssistantThreadMessageProps {
  messageId: string;
  onApplyQuery: (query: string) => void;
}

export const AssistantThreadMessage: React.FC<AssistantThreadMessageProps> = ({
  messageId,
  onApplyQuery,
}: AssistantThreadMessageProps) => {
  const message = useAssistantConversationStateActions((s) =>
    s.getMessageById(messageId)
  );
  const thinking = useAssistantConversationState((s) => s.thinking);

  const theme = useTheme2();

  const progressHeight =
    theme.spacing.gridSize * theme.components.height.sm * 0.75;
  const progressWidth =
    theme.spacing.gridSize * theme.components.height.sm * 1.5;

  const styles = getStyles(progressHeight, theme);

  const renderers: Partial<Components> = {
    pre: (props: any) => {
      const { children } = props;

      const element = Array.isArray(children)
        ? (props.children[0] as React.ReactElement)
        : (props.children as React.ReactElement);
      const className = element.props.className || "";
      const match = /language-(\w+)/.exec(className || "");
      return (
        <AssistantCode language={match?.[1]} onApply={onApplyQuery}>
          {String(element.props.children).trim()}
        </AssistantCode>
      );
    },
    code: (props: any) => {
      const { inline, className, children, ...rest } = props;
      return (
        <code className={styles.inlined} {...rest}>
          {children}
        </code>
      );
    },
    img: (props: any) => {
      const { src, alt, ...rest } = props;
      return <img src={src} alt={alt} className={styles.image} {...rest} />;
    },
  };

  const content = Array.isArray(message.content)
    ? message.content.join("")
    : message.content;

  const isUser = message.role === "user";
  const showProgress =
    thinking && message.role === "assistant" && message.status === "receiving";
  const showOnlyProgress = showProgress && message.content.length === 0;

  return (
    <div className={styles.container}>
      {!showOnlyProgress && (
        <div>
          <div className={styles.header}>
            <Icon size="lg" name={isUser ? "user" : "circle"} />
            <span>{isUser ? "You" : "Assistant"}</span>
            {isUser && message.status === "failed" && (
              <Tooltip content="Message failed to sent" placement="bottom">
                <Icon
                  size="lg"
                  name="exclamation-circle"
                  className={styles.error}
                />
              </Tooltip>
            )}
          </div>
          <div className={styles.content}>
            <ReactMarkdown remarkPlugins={[remarkGfm]} components={renderers}>
              {content}
            </ReactMarkdown>
          </div>
        </div>
      )}
      <div className={styles.footer}>
        {(showProgress || showOnlyProgress) && (
          <AssistantThinkingProgress
            height={progressHeight}
            width={progressWidth}
          />
        )}
      </div>
    </div>
  );
};

const getStyles = (progressHeight: number, theme: GrafanaTheme2) => {
  const padding = (theme.components as any).drawer?.padding ?? 2;
  const footerPaddingTop = 1;
  const footerHeight = Math.max(
    theme.spacing.gridSize * (padding * 3 - footerPaddingTop),
    progressHeight
  );

  return {
    container: css({
      display: "flex",
      flexDirection: "column",
      alignItems: "flex-start",
    }),
    header: css({
      display: "flex",
      alignItems: "center",
      alignSelf: "flex-start",
      columnGap: theme.spacing(1),
      fontWeight: theme.typography.fontWeightBold,
      fontSize: theme.typography.size.md,
      color: theme.colors.text.secondary,
    }),
    footer: css({
      display: "flex",
      alignItems: "start",
      paddingTop: theme.spacing(footerPaddingTop),
      height: `${footerHeight}px`,
      width: "100%",
    }),
    error: css({
      color: theme.colors.error.text,
    }),
    content: css({
      flex: 1,

      whiteSpace: "normal",
      fontSize: theme.typography.bodySmall.fontSize,
      lineHeight: theme.typography.bodySmall.lineHeight,
      color: theme.colors.text.primary,

      margin: 0,
      padding: 0,

      "& *, &": {
        border: 0,
        fontSize: "inherit",
        lineHeight: "inherit",
        margin: 0,
        padding: 0,
      },

      "& * + *, & > :first-child": {
        marginTop: theme.spacing(1),
      },

      "& h1, & h2": {
        fontSize: theme.typography.size.md,
        fontWeight: theme.typography.fontWeightBold,
        // margin: `${theme.spacing(2)} 0 ${theme.spacing(1)}`,
      },
      "& h2, & h3": {
        fontSize: theme.typography.size.md,
        fontWeight: theme.typography.fontWeightBold,
        // margin: `${theme.spacing(1.5)} 0 ${theme.spacing(0.75)}`,
      },
      "& h4, & h5, & h6": {
        fontSize: theme.typography.size.md,
        fontWeight: theme.typography.fontWeightBold,
        // margin: `${theme.spacing(1)} 0 ${theme.spacing(0.5)}`,
      },
      "& ul, & ol": {
        paddingLeft: theme.spacing(3),
      },
      "& ul li, & ol li": {
        // marginTop: theme.spacing(0.5),
      },
      "& blockquote": {
        borderLeft: `2px solid ${theme.colors.border.medium}`,
        paddingLeft: theme.spacing(2),
        // margin: `${theme.spacing(1)} 0`,
      },
      "& blockquote *": {
        color: theme.colors.text.secondary,
      },
      "& blockquote p": {
        // marginTop: theme.spacing(0.5),
      },
      "& blockquote ul, & blockquote ol": {
        paddingLeft: theme.spacing(3),
      },
      "& blockquote ul li, & blockquote ol li": {
        // marginTop: theme.spacing(0.5),
      },
      "& blockquote h1, & blockquote h2, & blockquote h3": {
        // margin: `${theme.spacing(1)} 0`,
      },
      "& a": {
        color: theme.colors.text.link,
        textDecoration: "underline",
      },
      "& img": {
        maxWidth: "100%",
        borderRadius: theme.shape.radius.default,
        // margin: `${theme.spacing(1)} 0`,
      },
      "& table": {
        width: "100%",
        borderCollapse: "collapse",
        // margin: `${theme.spacing(1)} 0`,
      },
      "& th, & td": {
        border: `1px solid ${theme.components.input.borderColor}`,
        padding: `${theme.spacing(0.5)} ${theme.spacing(1)}`,
        textAlign: "left",
      },
      "& th": {
        backgroundColor: theme.colors.background.primary,
        fontWeight: theme.typography.fontWeightMedium,
      },
      "& hr": {
        display: `none`,
      },
    }),
    inlined: css({
      backgroundColor: theme.colors.background.secondary,
      padding: "2px 4px",
      borderRadius: theme.shape.radius.default,
      fontFamily: theme.typography.fontFamilyMonospace,
      fontSize: "inherit",
    }),
    image: css({
      maxWidth: "100%",
      height: "auto",
      borderRadius: theme.shape.radius.default,
    }),
  };
};
