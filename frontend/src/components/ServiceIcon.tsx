import { useState } from "react";
import { useTheme } from "../hooks/theme-provider";
import { cn } from "../lib/utils";

type ServiceIconProps = {
  name: string;
  image?: string;
  imageLight?: string;
  imageDark?: string;
  /** `card` is the rounded-square grid logo; `table` is the round row avatar. */
  variant?: "card" | "table";
  className?: string;
};

function resolveIcon(
  { image, imageLight, imageDark }: Pick<ServiceIconProps, "image" | "imageLight" | "imageDark">,
  isDarkMode: boolean,
): string | undefined {
  if (isDarkMode) return imageDark || image || imageLight || undefined;
  return imageLight || image || imageDark || undefined;
}

/** "Dask Gateway" → "DG", "Keycloak" → "KE". */
function getServiceInitials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  if (words.length >= 2) return `${words[0][0]}${words[1][0]}`.toUpperCase();
  return (words[0] ?? "").slice(0, 2).toUpperCase();
}

export function ServiceIcon({
  name,
  image,
  imageLight,
  imageDark,
  variant = "card",
  className,
}: ServiceIconProps) {
  const { isDarkMode } = useTheme();
  const resolvedUrl = resolveIcon({ image, imageLight, imageDark }, isDarkMode);
  const [errorUrl, setErrorUrl] = useState<string | null>(null);
  const showImage = resolvedUrl !== undefined && errorUrl !== resolvedUrl;

  return (
    <div
      className={cn(
        "flex size-10 shrink-0 items-center justify-center overflow-hidden",
        variant === "card" ? "rounded-md" : "rounded-full",
        !showImage && "bg-muted text-muted-foreground-strong",
        !showImage && variant === "table" && "border border-border",
        className,
      )}
    >
      {showImage ? (
        <img
          src={resolvedUrl}
          alt=""
          aria-hidden="true"
          className="size-full object-contain"
          onError={() => setErrorUrl(resolvedUrl)}
        />
      ) : (
        <span
          aria-hidden="true"
          className={variant === "card" ? "text-sm font-medium" : "text-lg font-semibold leading-6"}
        >
          {getServiceInitials(name)}
        </span>
      )}
    </div>
  );
}
