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

export default function AppViewToggle(props: AppViewToggleProps): ReactNode {

  const { className, value, onChange } = props
  const isGrid = value === "grid";
  const isList = value === "list";
  
  return (
    <div
      className={cn("app-view-toggle", className)}
      role="radiogroup"
      aria-label="View mode"
    >
      <ButtonGroup>
        <Button
          type="button"
          role="radio"
          onClick={() => onChange("list")}
          className={cn("app-view-toggle__button", isList && "is-active")}
        >
          <List className="app-view-toggle__icon" size={18} aria-hidden="true" />
        </Button>

        <Button
          type="button"
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

export type AppViewToggleValue = "grid" | "list";

export type AppViewToggleProps = {
  className?: string;
  value: AppViewToggleValue;
  onChange: (next: AppViewToggleValue) => void;
  gridLabel?: string;
  listLabel?: string;
};
