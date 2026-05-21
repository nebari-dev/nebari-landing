import { useState } from "react"
import fallbackServiceImage from "../assets/Nebari.svg"
import { useTheme } from "../hooks/ThemeContext"

type IconUrl = string | { light?: string; dark?: string }

type ServiceIconProps = {
  imageUrl?: IconUrl
}

function resolveIcon(imageUrl: IconUrl | undefined, isDarkMode: boolean): string {
  if (!imageUrl) return fallbackServiceImage
  if (typeof imageUrl === "string") return imageUrl
  if (isDarkMode) return imageUrl.dark || imageUrl.light || fallbackServiceImage
  return imageUrl.light || imageUrl.dark || fallbackServiceImage
}

export function ServiceIcon({ imageUrl }: ServiceIconProps) {
  const { isDarkMode } = useTheme()
  const resolvedUrl = resolveIcon(imageUrl, isDarkMode)
  const [errorUrl, setErrorUrl] = useState<string | null>(null)
  const src = errorUrl === resolvedUrl ? fallbackServiceImage : resolvedUrl

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
  )
}
