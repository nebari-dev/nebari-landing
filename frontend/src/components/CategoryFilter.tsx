import { selectsAllCategories } from "../lib/services";
import { cn } from "../lib/utils";
import { Badge } from "./ui/badge";
import {
  Combobox,
  ComboboxChip,
  ComboboxChips,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  ComboboxValue,
} from "./ui/combobox";

const MAX_VISIBLE_CHIPS = 2;

type CategoryFilterProps = {
  categories: string[];
  value: string[];
  onChange: (value: string[]) => void;
  className?: string;
};

/**
 * Multi-select category filter; an empty selection shows every category.
 * Chips follow menu order on a single row, at most two before a "+N" count,
 * and a selection of every category reads as "All categories".
 */
export function CategoryFilter({ categories, value, onChange, className }: CategoryFilterProps) {
  const allSelected = selectsAllCategories(categories, value);

  return (
    <Combobox multiple items={categories} value={value} onValueChange={onChange}>
      <ComboboxChips
        className={cn(
          "flex-nowrap overflow-hidden",
          value.length > 0 && "*:data-[slot=combobox-input]:min-w-2",
        )}
        fieldClassName={cn("min-w-0 flex-nowrap", className)}
      >
        <ComboboxValue>
          {(selected: string[]) => {
            if (allSelected) {
              return <span className="truncate text-sm text-foreground">All categories</span>;
            }

            const ordered = categories.filter((category) => selected.includes(category));
            const hidden = ordered.slice(MAX_VISIBLE_CHIPS);
            return (
              <>
                {ordered.slice(0, MAX_VISIBLE_CHIPS).map((category) => (
                  <ComboboxChip key={category}>{category}</ComboboxChip>
                ))}
                {hidden.length > 0 && (
                  <Badge variant="secondary" title={hidden.join(", ")}>
                    <span aria-hidden="true">+{hidden.length}</span>
                    <span className="sr-only">{`${hidden.length} more selected: ${hidden.join(", ")}`}</span>
                  </Badge>
                )}
              </>
            );
          }}
        </ComboboxValue>
        <ComboboxInput
          aria-label="Filter by category"
          placeholder={value.length === 0 ? "All categories" : undefined}
          className="w-0 placeholder:text-muted-foreground-strong"
        />
      </ComboboxChips>
      <ComboboxContent align="start">
        <ComboboxEmpty>No categories found.</ComboboxEmpty>
        <ComboboxList>
          {(category: string) => (
            <ComboboxItem key={category} value={category}>
              {category}
            </ComboboxItem>
          )}
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  );
}
