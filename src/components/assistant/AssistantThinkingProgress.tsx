import React, { useRef } from "react";
import { css, keyframes } from "@emotion/css";
import { useTheme2 } from "@grafana/ui";
import { transparentize } from "polished";
import { GrafanaTheme2 } from "@grafana/data";

interface AssistantWaveProgressProps {
  barCount?: number;
  width?: number;
  height?: number;
  gap?: number;
  speed?: number;
  randomness?: number;
}

export const AssistantThinkingProgress: React.FC<
  AssistantWaveProgressProps
> = ({ barCount = 6, width = 34, height = 15, gap = 2, speed = 0.8 }) => {
  let gapWidth = (barCount - 1) * gap;
  const widthWithoutGaps = width - gapWidth;
  width = Math.trunc(widthWithoutGaps / barCount) * barCount + gapWidth;

  const theme = useTheme2();
  const styles = getStyles(gap, width, height, theme);

  const backgroundColor = theme.isLight
    ? transparentize(0.35, theme.colors.text.secondary)
    : theme.colors.text.secondary;

  const offsets = useRef<number[]>(
    Array.from({ length: barCount }, () => Math.random() * speed)
  );

  const wave = keyframes`
        0%, 100% { transform: scaleY(0.3); }
        50%      { transform: scaleY(1); }
    `;

  return (
    <div
      className={styles.container}
      role="img"
      aria-label="Assistant thinking"
    >
      {offsets.current.map((offset, i) => {
        const bar = css({
          flex: 1,
          height: "100%",
          backgroundColor,
          borderRadius: "2px",
          transformOrigin: "center bottom",
          animation: `${wave} ${speed}s ease-in-out infinite`,
          animationDelay: `${-offset}s`,
        });
        return <div key={i} className={bar} />;
      })}
    </div>
  );
};

const getStyles = (
  gap: number,
  width: number,
  height: number,
  theme: GrafanaTheme2
) => ({
  container: css({
    display: "flex",
    alignItems: "flex-end",
    gap: `${gap}px`,
    width: `${width}px`,
    height: `${height}px`,
    marginBottom: `${gap}px`,
  }),
});
