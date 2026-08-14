import { useState } from "react";
import fallbackServiceImage from "../assets/Nebari.svg";
import { useTheme } from "../hooks/theme-provider";

type ServiceIconProps = {
  image?: string;
  imageLight?: string;
  imageDark?: string;
};

function resolveIcon(
  { image, imageLight, imageDark }: ServiceIconProps,
  isDarkMode: boolean,
): string {
  if (isDarkMode) return imageDark || image || imageLight || fallbackServiceImage;
  return imageLight || image || imageDark || fallbackServiceImage;
}

export function ServiceIcon({ image, imageLight, imageDark }: ServiceIconProps) {
  const { isDarkMode } = useTheme();
  const resolvedUrl = resolveIcon({ image, imageLight, imageDark }, isDarkMode);
  const [errorUrl, setErrorUrl] = useState<string | null>(null);
  const src = errorUrl === resolvedUrl ? fallbackServiceImage : resolvedUrl;

  return (
    <div className="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-[10px] bg-muted">
      <img
        src={src}
        alt=""
        aria-hidden="true"
        className="h-9 w-9 object-contain"
        onError={() => setErrorUrl(resolvedUrl)}
      />
    </div>
  );
}
