import React from "react";
import "./index.scss";

export type AppGridProps = {
  className?: string;
  style?: React.CSSProperties;
  children: React.ReactNode;
};

function cx(...parts: Array<string | undefined | false>) {
  return parts.filter(Boolean).join(" ");
}

export default function AppGrid({
  className,
  style,
  children,
}: AppGridProps): React.ReactNode {
  return (
    <ul className={cx("app-grid", className)} style={style}>
      {children}
    </ul>
  );
}
