import { type ReactNode, useEffect, useId, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Field, FieldLabel } from "@/components/ui/field";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { EmptyState } from "./EmptyState";

export type PickerOption = { value: string; label: string; description?: string };

type PickerDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: ReactNode;
  fieldLabel: string;
  options: PickerOption[];
  emptyTitle?: string;
  confirmLabel?: string;
  pending?: boolean;
  onConfirm: (value: string) => void;
};

/** Modal with a single select; used for "add to group…" / "assign role…". */
export function PickerDialog({
  open,
  onOpenChange,
  title,
  description,
  fieldLabel,
  options,
  emptyTitle = "Nothing to choose from",
  confirmLabel = "Apply",
  pending = false,
  onConfirm,
}: PickerDialogProps) {
  const [value, setValue] = useState<string | null>(null);
  const id = useId();

  useEffect(() => {
    if (!open) setValue(null);
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description ? <DialogDescription>{description}</DialogDescription> : null}
        </DialogHeader>

        {options.length === 0 ? (
          <EmptyState title={emptyTitle} />
        ) : (
          <Field>
            <FieldLabel htmlFor={id}>{fieldLabel}</FieldLabel>
            <Select value={value} onValueChange={(v) => setValue(v as string | null)}>
              <SelectTrigger id={id} className="w-full">
                <SelectValue placeholder="Select…">
                  {(v: string | null) => options.find((o) => o.value === v)?.label ?? "Select…"}
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                {options.map((o) => (
                  <SelectItem key={o.value} value={o.value}>
                    <span className="flex flex-col">
                      <span>{o.label}</span>
                      {o.description ? (
                        <span className="text-xs text-muted-foreground">{o.description}</span>
                      ) : null}
                    </span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
        )}

        <DialogFooter>
          <DialogClose render={<Button variant="outline" />}>Cancel</DialogClose>
          <Button
            disabled={value === null || options.length === 0}
            loading={pending}
            onClick={() => value !== null && onConfirm(value)}
          >
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
