import type {
  ReactNode
} from "react";

import "./index.scss";

export default function AvatarImage(imageSrc: string): ReactNode {
  
  return (
    <img
      src={imageSrc}
      alt="Img"
      className="app-avatar-image"
      width={48}
      height={48}
      loading="lazy"
    />
  );
}
