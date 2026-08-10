import { type ReactNode, useEffect, useRef } from "react";
import { type BannerConfig, safeCssValue } from "../app/config";
import { cn } from "../lib/utils";

export type BannerProps = {
  /**
   * Which edge of the viewport the banner is pinned to. Also selects the CSS
   * variable (--top-banner-height / --bottom-banner-height) the rest of the
   * layout uses to make room for it.
   */
  position: "top" | "bottom";
  config?: BannerConfig;
};

/**
 * Full-width classification banner (e.g. CUI) pinned to the top or bottom of
 * the viewport. Configured at runtime via /config.json — see
 * AppConfig.banners. Renders nothing when no text is configured.
 *
 * Text is rendered as plain text (never HTML). Color values pass through the
 * same UNSAFE_CSS guard as theme tokens; unsafe values fall back to the
 * theme's inverted colors, which follow light/dark mode.
 */
export function Banner({ position, config }: BannerProps): ReactNode {
  const bannerRef = useRef<HTMLDivElement>(null);
  const hasText = Boolean(config?.text);

  // Publish the rendered height so the page offsets (which consume the
  // variable with a 0px fallback) shift to make room. ResizeObserver keeps
  // it accurate when the text wraps on resize.
  useEffect(() => {
    const element = bannerRef.current;
    const root = document.documentElement;
    const heightVariable = `--${position}-banner-height`;
    if (!element) {
      return;
    }
    const updateHeight = () => {
      root.style.setProperty(heightVariable, `${element.offsetHeight}px`);
    };
    updateHeight();
    const observer = new ResizeObserver(updateHeight);
    observer.observe(element);
    return () => {
      observer.disconnect();
      root.style.removeProperty(heightVariable);
    };
  }, [position, hasText]);

  if (!config?.text) return null;

  const background = safeCssValue(config.background);
  const foreground = safeCssValue(config.foreground);

  return (
    <div
      ref={bannerRef}
      role="note"
      className={cn(
        "fixed inset-x-0 z-[60] w-full py-1 text-center text-sm font-semibold",
        position === "top" ? "top-0" : "bottom-0",
        !background && "bg-foreground",
        !foreground && "text-background",
      )}
      style={{ backgroundColor: background, color: foreground }}
    >
      {config.text}
    </div>
  );
}
