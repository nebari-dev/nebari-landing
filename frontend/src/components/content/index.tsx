import type { ReactNode } from "react";

import AppAccordion from "../accordion";
import type { Service } from "../../api/listServices";

import "./index.scss";

type ContentProps = {
  services?: Service[] | null;
};

export default function Content({ services }: ContentProps): ReactNode {
  const serviceList = services ?? [];
  const pinnedServices = serviceList.filter((service) => service.pinned);

  return (
    <main id="main-content" className="app-content">
      <h1 className="launchpad-title">Launchpad</h1>

      <AppAccordion
        pinnedServices={pinnedServices}
        services={serviceList}
      />
    </main>
  );
}
