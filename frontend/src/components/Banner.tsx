import type { ReactNode } from "react";
import { type BannerConfig, safeCssValue } from "../app/config";

export type BannerProps = {
  config?: BannerConfig;
};

/**
 * Full-width classification banner (e.g. CUI) rendered above the header or
 * below the page content. Configured at runtime via /config.json — see
 * AppConfig.banners. Renders nothing when no text is configured.
 *
 * Text is rendered as plain text (never HTML). Color values pass through the
 * same UNSAFE_CSS guard as theme tokens; unsafe values fall back to the
 * theme's inverted colors, which follow light/dark mode.
 */
export function Banner({ config }: BannerProps): ReactNode {
  if (!config?.text) return null;

  const background = safeCssValue(config.background);
  const foreground = safeCssValue(config.foreground);

  return (
    <div
      role="note"
      className="w-full bg-foreground py-1 text-center text-sm font-semibold text-background"
      style={{ backgroundColor: background, color: foreground }}
    >
      {config.text}
    </div>
  );
}
