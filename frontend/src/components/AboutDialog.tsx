import { FileText, X } from "lucide-react";
import { type ReactNode, useEffect, useState } from "react";
import { type BuildInfo, getBuildInfo } from "../api/buildInfo";
import type { Service } from "../api/listServices";
import builtInLogoDark from "../assets/nebari-logo_dark.svg";
import builtInLogoLight from "../assets/nebari-logo_light.svg";
import { GithubIcon } from "./icons/GithubIcon";
import { Badge } from "./ui/badge";
import { Button } from "./ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./ui/dialog";

type AboutDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  isDarkMode?: boolean;
  logoSrc?: string;
  logoSrcDark?: string;
  services?: Service[];
  environment?: string;
};

export function AboutDialog({
  open,
  onOpenChange,
  isDarkMode = false,
  logoSrc,
  logoSrcDark,
  services = [],
  environment,
}: AboutDialogProps): ReactNode {
  const [buildInfo, setBuildInfo] = useState<BuildInfo | null>(null);
  const [copyStatus, setCopyStatus] = useState<"idle" | "copied" | "error">("idle");

  useEffect(() => {
    if (!open) {
      setCopyStatus("idle");
      return;
    }

    const controller = new AbortController();
    getBuildInfo(controller.signal)
      .then(setBuildInfo)
      .catch((error: unknown) => {
        if (!(error instanceof DOMException && error.name === "AbortError")) {
          console.error("Unable to load build information", error);
        }
      });

    return () => controller.abort();
  }, [open]);

  const nebariLogo = isDarkMode
    ? (logoSrcDark ?? logoSrc ?? builtInLogoDark)
    : (logoSrc ?? builtInLogoLight);

  const handleCopyAll = async () => {
    try {
      await navigator.clipboard.writeText(formatSetup(buildInfo, environment, services));
      setCopyStatus("copied");
    } catch {
      setCopyStatus("error");
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="max-h-[calc(100vh-2rem)] max-w-[534px] gap-0 rounded-lg bg-card p-0"
        showCloseButton={false}
      >
        <DialogHeader className="flex flex-row items-center justify-between border-b border-border p-3">
          <div>
            <DialogTitle className="text-base leading-5">About Nebari</DialogTitle>
            <DialogDescription className="sr-only">
              Build, platform, software pack, and service information for this Nebari deployment.
            </DialogDescription>
          </div>
          <DialogClose render={<Button type="button" variant="ghost" size="icon" />}>
            <X aria-hidden="true" />
            <span className="sr-only">Close</span>
          </DialogClose>
        </DialogHeader>

        <div className="min-h-0 bg-background px-3">
          <div className="flex max-h-[632px] flex-col gap-6 overflow-y-auto p-6">
            <div className="flex items-center gap-3 pb-1">
              <img src={nebariLogo} alt="Nebari" className="h-7 w-auto max-w-[108px]" />
              <div className="min-w-0 flex-1 text-xs leading-4">
                <p className="font-medium text-foreground">Nebari Core</p>
                <p className="truncate text-muted-foreground">nebari-infrastructure-core</p>
              </div>
              <Badge variant="secondary" className="px-1.5">
                {formatVersion(buildInfo?.version)}
              </Badge>
            </div>

            <AboutSection title="Platform">
              <AboutRow label="Commit" value={buildInfo?.commit ?? "—"} />
              <AboutRow label="Last updated" value={formatDate(buildInfo?.lastUpdated)} />
              <AboutRow label="Environment" value={environment || "—"} last />
            </AboutSection>

            <AboutSection title="Software packs">
              <AboutRow label="No software pack information available" value="—" last />
            </AboutSection>

            <AboutSection title="Services">
              {services.length > 0 ? (
                services.map((service, index) => (
                  <AboutRow
                    key={service.id}
                    label={
                      <span className="flex items-center gap-2">
                        <span
                          data-slot="service-status-dot"
                          className="size-2 rounded-full"
                          style={{ backgroundColor: getStatusDotColor(service.status) }}
                          aria-hidden="true"
                        />
                        {service.name}
                      </span>
                    }
                    value={formatVersion(service.version)}
                    strong
                    last={index === services.length - 1}
                  />
                ))
              ) : (
                <AboutRow label="No services available" value="—" last />
              )}
            </AboutSection>
          </div>
        </div>

        <DialogFooter className="flex-row items-center justify-between border-t border-border p-3 sm:justify-between">
          <div className="flex items-center gap-2">
            <Button
              render={
                <a
                  href="https://github.com/nebari-dev/nebari-landing/issues/new"
                  target="_blank"
                  rel="noreferrer"
                />
              }
              variant="ghost"
              size="sm"
            >
              <GithubIcon />
              Report an issue
            </Button>
            <Button
              render={<a href="https://nebari.dev/docs/welcome" target="_blank" rel="noreferrer" />}
              variant="ghost"
              size="sm"
            >
              <FileText aria-hidden="true" />
              Documentation
            </Button>
          </div>
          <Button type="button" variant="outline" size="sm" onClick={handleCopyAll}>
            {copyStatus === "copied"
              ? "Copied"
              : copyStatus === "error"
                ? "Copy failed"
                : "Copy all"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function AboutSection({ title, children }: { title: string; children: ReactNode }): ReactNode {
  return (
    <section className="flex flex-col gap-2">
      <h3 className="text-sm font-medium uppercase leading-5 text-muted-foreground">{title}</h3>
      <div className="overflow-hidden rounded-md border border-border bg-card">{children}</div>
    </section>
  );
}

function AboutRow({
  label,
  value,
  strong = false,
  last = false,
}: {
  label: ReactNode;
  value: ReactNode;
  strong?: boolean;
  last?: boolean;
}): ReactNode {
  return (
    <div
      className={`flex min-h-11 items-center justify-between gap-4 px-4 py-3 text-sm leading-5 ${
        last ? "" : "border-b border-border"
      }`}
    >
      <div className={strong ? "font-medium text-foreground" : "text-(--text-secondary)"}>
        {label}
      </div>
      <div className="shrink-0 text-right text-muted-foreground">{value}</div>
    </div>
  );
}

function formatVersion(version?: string): string {
  if (!version || version === "dev") return version ?? "—";
  return version.startsWith("v") ? version : `v${version}`;
}

function formatDate(value?: string | null): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  }).format(date);
}

function getStatusDotColor(status: string): string {
  const normalizedStatus = status.toLowerCase();
  if (normalizedStatus === "healthy") return "var(--status-healthy-dot)";
  if (normalizedStatus === "unhealthy") return "var(--status-unhealthy-dot)";
  return "var(--status-default-dot)";
}

function formatSetup(
  buildInfo: BuildInfo | null,
  environment: string | undefined,
  services: Service[],
): string {
  const serviceLines =
    services.length > 0
      ? services.map(
          (service) => `${service.name} (${service.status}): ${formatVersion(service.version)}`,
        )
      : ["No services available: —"];

  return [
    "Nebari Core",
    "Repository: nebari-infrastructure-core",
    `Version: ${formatVersion(buildInfo?.version)}`,
    "",
    "PLATFORM",
    `Commit: ${buildInfo?.commit ?? "—"}`,
    `Last updated: ${formatDate(buildInfo?.lastUpdated)}`,
    `Environment: ${environment || "—"}`,
    "",
    "SOFTWARE PACKS",
    "No software pack information available: —",
    "",
    "SERVICES",
    ...serviceLines,
  ].join("\n");
}
