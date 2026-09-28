import { useId } from "react";
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
} from "@/components/ui/combobox";
import { Label } from "@/components/ui/label";
import type { AdminUser } from "../api/types";
import { displayName } from "../lib/format";

type UserPickerProps = {
  label: string;
  users: AdminUser[];
  value: AdminUser[];
  onChange: (users: AdminUser[]) => void;
  placeholder?: string;
};

/** Multi-select combobox of users, rendered as removable chips. */
export function UserPicker({
  label,
  users,
  value,
  onChange,
  placeholder = "Search by name, username or email…",
}: UserPickerProps) {
  const id = useId();
  return (
    <div className="flex flex-col gap-2">
      <Label htmlFor={id}>{label}</Label>
      <Combobox
        multiple
        items={users}
        value={value}
        onValueChange={(next) => onChange(next as AdminUser[])}
        itemToStringLabel={(u: AdminUser) => `${displayName(u)} ${u.username} ${u.email}`}
        isItemEqualToValue={(a: AdminUser, b: AdminUser) => a.id === b.id}
      >
        <ComboboxChips>
          <ComboboxValue>
            {(selected: AdminUser[]) =>
              selected.map((u) => (
                <ComboboxChip key={u.id} aria-label={displayName(u)}>
                  {displayName(u)}
                </ComboboxChip>
              ))
            }
          </ComboboxValue>
          <ComboboxInput id={id} placeholder={placeholder} />
        </ComboboxChips>
        <ComboboxContent>
          <ComboboxEmpty>No matching users.</ComboboxEmpty>
          <ComboboxList>
            {(u: AdminUser) => (
              <ComboboxItem key={u.id} value={u}>
                <span className="flex flex-col">
                  <span>{displayName(u)}</span>
                  <span className="text-xs text-muted-foreground">{u.email}</span>
                </span>
              </ComboboxItem>
            )}
          </ComboboxList>
        </ComboboxContent>
      </Combobox>
    </div>
  );
}
