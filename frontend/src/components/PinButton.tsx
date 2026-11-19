import { Pin, PinOff } from "lucide-react";
import { cn } from "../lib/utils";
import { Button } from "./ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "./ui/tooltip";

type PinButtonProps = {
  pinned: boolean;
  onToggle: (nextPinned: boolean) => void;
  className?: string;
};

/** Pin toggle for a service card or table row. */
export function PinButton({ pinned, onToggle, className }: PinButtonProps) {
  const label = pinned ? "Unpin service" : "Pin service";

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={label}
            className={cn(
              "group/pin text-foreground hover:bg-muted focus-visible:ring-offset-0",
              className,
            )}
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              onToggle(!pinned);
            }}
          />
        }
      >
        <Pin
          strokeWidth={1.5}
          className={cn(
            "size-4",
            pinned &&
              "fill-current text-primary group-hover/pin:hidden group-focus-visible/pin:hidden",
          )}
        />
        {pinned && (
          <PinOff
            aria-hidden="true"
            strokeWidth={1.5}
            className="hidden size-4 group-hover/pin:block group-focus-visible/pin:block"
          />
        )}
      </TooltipTrigger>
      <TooltipContent side="bottom" showArrow={false}>
        {pinned ? "Unpin" : "Pin"}
      </TooltipContent>
    </Tooltip>
  );
}
