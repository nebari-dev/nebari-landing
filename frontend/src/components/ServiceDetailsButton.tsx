import { cn } from "../lib/utils";
import { Button } from "./ui/button";

type ServiceDetailsButtonProps = {
  serviceName: string;
  /** Fades the button for an unavailable service. */
  muted?: boolean;
  onClick?: () => void;
  className?: string;
};

/** Opens a service's details without opening the service itself. */
export function ServiceDetailsButton({
  serviceName,
  muted = false,
  onClick,
  className,
}: ServiceDetailsButtonProps) {
  return (
    <Button
      variant="ghost"
      size="xs"
      aria-label={`${serviceName} details`}
      className={cn(
        "px-2 text-foreground hover:bg-muted hover:no-underline focus-visible:ring-offset-0",
        muted && "opacity-60",
        className,
      )}
      onClick={(event) => {
        event.preventDefault();
        event.stopPropagation();
        onClick?.();
      }}
    >
      Details
    </Button>
  );
}
