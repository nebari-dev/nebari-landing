import { ArrowUpRight } from "lucide-react";
import type { ReactNode } from "react";
import type { Service } from "../api/listServices";

type ServiceDetailsMetadataProps = {
  service: Service;
};

/** "argo-cd.readthedocs.io/en/stable" → "argo-cd.readthedocs.io"; falls back to the raw value. */
function getHostname(url: string) {
  try {
    return new URL(url).hostname;
  } catch {
    return url;
  }
}

/** Category, version, and docs link of a service; missing values are omitted. */
export function ServiceDetailsMetadata({ service }: ServiceDetailsMetadataProps) {
  const rows: { label: string; value: ReactNode }[] = [];

  if (service.category.length > 0) {
    rows.push({ label: "Category", value: service.category.join(", ") });
  }
  if (service.version) {
    rows.push({ label: "Version", value: service.version });
  }
  if (service.docsUrl) {
    rows.push({
      label: "Docs",
      value: (
        <a
          href={service.docsUrl}
          target="_blank"
          rel="noreferrer"
          className="inline-flex items-center gap-0.5 rounded-sm font-medium text-primary no-underline outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
        >
          {getHostname(service.docsUrl)}
          <ArrowUpRight aria-hidden="true" strokeWidth={1.5} className="size-3.5" />
          <span className="sr-only">(opens in a new tab)</span>
        </a>
      ),
    });
  }

  if (rows.length === 0) return null;

  return (
    <dl className="text-[13px] leading-[19px]">
      {rows.map(({ label, value }) => (
        <div key={label} className="flex items-center border-t border-border py-2.5">
          <dt className="w-30 shrink-0 text-muted-foreground">{label}</dt>
          <dd className="min-w-0 break-words text-foreground">{value}</dd>
        </div>
      ))}
    </dl>
  );
}
