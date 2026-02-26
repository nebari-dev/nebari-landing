import type { ReactNode } from "react";

import {
  ButtonGroup, Button
} from "@trussworks/react-uswds";

import {
  List, LayoutGrid 
} from "lucide-react";

import {
  cn
} from "../../lib/cn";

import "./index.scss";

export default function AppViewToggle(props: AppViewToggle.Props): ReactNode {

  const { className, value, onChange } = props
  const isGrid = value === "grid";
  const isList = value === "list";
  
  return (
    <div
      className={cn("app-view-toggle", className)}
      role="radiogroup"
      aria-label="View mode"
    >
      <ButtonGroup type="segmented" className="app-view-toggle__group">
        <Button
          type="button"
          unstyled
          role="radio"
          onClick={() => onChange("list")}
          className={cn("app-view-toggle__button", isList && "is-active")}
        >
          <List className="app-view-toggle__icon" size={18} aria-hidden="true" />
        </Button>

        <Button
          type="button"
          unstyled
          role="radio"
          onClick={() => onChange("grid")}
          className={cn("app-view-toggle__button", isGrid && "is-active")}
        >
          <LayoutGrid className="app-view-toggle__icon" size={18} aria-hidden="true" />
        </Button>
      </ButtonGroup>
    </div>
  );
}

export namespace AppViewToggle {
  export type Value = "grid" | "list";

  export type Props = {
    className?: string;
    value: Value;
    onChange: (next: Value) => void;
    gridLabel?: string;
    listLabel?: string;
  };
}
