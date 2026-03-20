import { useState } from "react"
import fallbackServiceImage from "../assets/Nebari.svg"

type ServiceIconProps = {
  imageUrl?: string
}

export function ServiceIcon({ imageUrl }: ServiceIconProps) {
  const [src, setSrc] = useState(imageUrl || fallbackServiceImage)

  return (
    <div className="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-[10px] bg-muted">
      <img
        src={src}
        alt=""
        aria-hidden="true"
        className="h-9 w-9 object-contain"
        onError={() => setSrc(fallbackServiceImage)}
      />
    </div>
  )
}
