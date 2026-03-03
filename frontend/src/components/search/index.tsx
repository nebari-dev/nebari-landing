import type {
  ReactNode
} from "react";

import {
  Search
} from "@trussworks/react-uswds";

import {
  cn
} from "../../lib/cn";

import "./index.scss";

export default function AppSearchBar(props: AppSearchBarProps): ReactNode {
  const { className, onSubmit } = props;

  return (
    <Search
      className={cn("usa-search app-search", className)}
      size="small"
      placeholder="Search"
      onSubmit={(event) => {
        event.preventDefault();

        const form = event.currentTarget as HTMLFormElement;
        const input = form.querySelector('input[type="search"]') as HTMLInputElement | null;

        onSubmit?.(input?.value ?? "");
      }}
    />
  );
}

export type AppSearchBarProps = {
  className?: string;
  onSubmit?: (value: string) => void;
};
