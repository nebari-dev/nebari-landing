import type { ServicesView } from "../lib/preferences";
import { Button } from "./ui/button";
import { ButtonGroup } from "./ui/button-group";

export type { ServicesView };

const VIEWS: { value: ServicesView; label: string }[] = [
  { value: "grid", label: "Grid" },
  { value: "table", label: "Table" },
];

type ViewToggleProps = {
  view: ServicesView;
  onChange: (view: ServicesView) => void;
};

/** Grid / Table view switch. */
export function ViewToggle({ view, onChange }: ViewToggleProps) {
  return (
    <ButtonGroup aria-label="View">
      {VIEWS.map(({ value, label }) => (
        <Button
          key={value}
          variant="outline"
          aria-pressed={view === value}
          className="bg-transparent px-2.5 text-muted-foreground-strong shadow-none hover:no-underline aria-pressed:z-10 aria-pressed:border-border-strong aria-pressed:bg-muted aria-pressed:text-foreground"
          onClick={() => onChange(value)}
        >
          {label}
        </Button>
      ))}
    </ButtonGroup>
  );
}
