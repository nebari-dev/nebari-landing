import { renderWithProviders as render } from "@/test/render";
import { screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ServicesAccordion } from "@/components/ServiceAccordion";

const services = [
  {
    id: "svc-1",
    name: "JupyterHub",
    status: "Healthy",
    description: "Notebook platform",
    category: ["data"],
    pinned: true,
    image: "",
    url: "https://example.com/jupyterhub",
    priority: 100,
  },
  {
    id: "svc-2",
    name: "Grafana",
    status: "Unhealthy",
    description: "Metrics dashboards",
    category: ["observability"],
    pinned: false,
    image: "",
    url: "https://example.com/grafana",
    priority: 100,
  },
];

function renderAccordion(pinnedServices = services.filter((service) => service.pinned)) {
  return render(
    <ServicesAccordion
      pinnedServices={pinnedServices}
      otherServices={services.filter((service) => !service.pinned)}
      onTogglePin={vi.fn()}
    />,
  );
}

describe("ServicesAccordion", () => {
  it("renders the pinned and all services sections with their counts", () => {
    renderAccordion();

    expect(screen.getByRole("button", { name: "Pinned 1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "All services 1" })).toBeInTheDocument();
  });

  it("renders pinned services as cards in the pinned section", () => {
    renderAccordion();

    const pinned = screen.getByRole("region", { name: /Pinned/ });
    expect(within(pinned).getByText("JupyterHub")).toBeInTheDocument();
    expect(within(pinned).queryByText("Grafana")).not.toBeInTheDocument();
  });

  it("does not repeat pinned services in the all services section", () => {
    renderAccordion();

    const all = screen.getByRole("region", { name: /All services/ });
    expect(within(all).getByText("Grafana")).toBeInTheDocument();
    expect(within(all).queryByText("JupyterHub")).not.toBeInTheDocument();
  });

  it("leaves out the pinned section when nothing is pinned", () => {
    renderAccordion([]);

    expect(screen.queryByRole("button", { name: /^Pinned/ })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "All services 1" })).toBeInTheDocument();
  });
});
