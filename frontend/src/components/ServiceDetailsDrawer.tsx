import { XIcon } from "lucide-react";
import { type CSSProperties, useLayoutEffect, useRef, useState } from "react";
import type { Service } from "../api/listServices";
import { ServiceDetailsMetadata } from "./ServiceDetailsMetadata";
import { ServiceIcon } from "./ServiceIcon";
import { Button } from "./ui/button";
import {
  Drawer,
  DrawerBody,
  DrawerClose,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "./ui/drawer";

type ServiceDetailsDrawerProps = {
  service: Service | undefined;
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

/** The viewport offset of the page header's bottom edge, so the drawer opens below it. */
function headerBottom() {
  return Math.max(0, document.querySelector("header")?.getBoundingClientRect().bottom ?? 0);
}

/** Side drawer with a service's full description, category, version, and docs link. */
export function ServiceDetailsDrawer({ service, open, onOpenChange }: ServiceDetailsDrawerProps) {
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const [top, setTop] = useState(0);

  useLayoutEffect(() => {
    if (open) setTop(headerBottom());
  }, [open]);

  return (
    <Drawer open={open && service !== undefined} onOpenChange={onOpenChange}>
      <DrawerContent
        showCloseButton={false}
        initialFocus={closeButtonRef}
        portalProps={{ style: { "--drawer-top": `${top}px` } as CSSProperties }}
        overlayClassName="top-(--drawer-top)"
        viewportClassName="top-(--drawer-top)"
        className="data-[swipe-axis=x]:top-(--drawer-top) data-[swipe-axis=x]:h-[calc(100dvh-var(--drawer-top))]"
      >
        {service && (
          <>
            <DrawerHeader className="pr-4">
              <div className="flex min-w-0 flex-1 items-center gap-3">
                <ServiceIcon
                  name={service.name}
                  image={service.image}
                  imageLight={service.imageLight}
                  imageDark={service.imageDark}
                />
                <DrawerTitle className="min-w-0 break-words">{service.name}</DrawerTitle>
              </div>
              <DrawerClose
                ref={closeButtonRef}
                render={
                  <Button
                    variant="ghost"
                    size="icon"
                    className="hover:bg-muted hover:no-underline"
                  />
                }
              >
                <XIcon aria-hidden="true" strokeWidth={1.5} className="size-4.5" />
                <span className="sr-only">Close</span>
              </DrawerClose>
            </DrawerHeader>

            <DrawerBody className="gap-5 px-8 pt-8 pb-4">
              {service.description && (
                <DrawerDescription className="whitespace-pre-line text-muted-foreground-strong">
                  {service.description}
                </DrawerDescription>
              )}
              <ServiceDetailsMetadata service={service} />
            </DrawerBody>
          </>
        )}
      </DrawerContent>
    </Drawer>
  );
}
