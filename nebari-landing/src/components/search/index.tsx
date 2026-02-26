import {
  useState,
} from "react";

import type {
  FormEvent, ReactNode
} from "react";

import {
  Search as SearchIcon
} from "lucide-react";

import {
  cn
} from "../../lib/cn";

import "./index.scss";

export default function AppSearchBar(props: AppSearchBar.Props): ReactNode {

  // Extract props
  const {className, onSubmit} = props;

  // search value
  const [value, setValue] = useState("");

  // submit behavior
  const handleSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    onSubmit?.(value);
  };

  return (
    <form
      className={cn("app-search", className)}
      role="search"
      onSubmit={handleSubmit}
    >
      <label className="usa-sr-only" htmlFor="app-search-input">
        Search
      </label>

      <input
        id="app-search-input"
        className="usa-input app-search__input"
        type="search"
        value={value}
        placeholder="Search"
        onChange={(e) => setValue(e.target.value)}
      />

      <button
        type="submit"
        className="usa-button app-search__button"
        aria-label="Search"
      >
        <SearchIcon size={18} className="app-search__icon" aria-hidden="true" />
        <span className="usa-sr-only">Search</span>
      </button>
    </form>
  );
}

export
namespace AppSearchBar {

  export
  type Props = {
    className?: string;
    onSubmit?: (value: string) => void;
  };
}
