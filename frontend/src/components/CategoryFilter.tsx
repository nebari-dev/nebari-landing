import { cn } from "../lib/utils";
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

type CategoryFilterProps = {
  categories: string[];
  value: string[];
  onChange: (value: string[]) => void;
  className?: string;
};

/** Multi-select category filter; an empty selection shows every category. */
export function CategoryFilter({ categories, value, onChange, className }: CategoryFilterProps) {
  return (
    <Combobox multiple items={categories} value={value} onValueChange={onChange}>
      <ComboboxChips fieldClassName={cn("min-w-0", className)}>
        <ComboboxValue>
          {(selected: string[]) =>
            selected.map((category) => <ComboboxChip key={category}>{category}</ComboboxChip>)
          }
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
