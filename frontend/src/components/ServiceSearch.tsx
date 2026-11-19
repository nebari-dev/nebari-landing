import { Search } from "lucide-react";
import { useEffect, useRef } from "react";
import { cn } from "../lib/utils";
import { Input } from "./ui/input";

type ServiceSearchProps = {
  value: string;
  onChange: (value: string) => void;
  className?: string;
};

/** Search field; Ctrl/⌘+K focuses it from anywhere on the page. */
export function ServiceSearch({ value, onChange, className }: ServiceSearchProps) {
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const isShortcut = (event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k";

      if (!isShortcut) return;

      event.preventDefault();
      inputRef.current?.focus();
      inputRef.current?.select();
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  return (
    <div className={cn("relative min-w-0", className)}>
      <Search
        aria-hidden="true"
        strokeWidth={1.5}
        className="pointer-events-none absolute top-1/2 left-3 z-10 size-4.5 -translate-y-1/2 text-muted-foreground"
      />
      <Input
        ref={inputRef}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder="Search services…"
        aria-label="Search services"
        className="pl-9.5 leading-5"
      />
    </div>
  );
}
