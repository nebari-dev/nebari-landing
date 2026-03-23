import { render, screen } from "@testing-library/react";
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
  },
];

describe("ServicesAccordion", () => {
  it("renders the pinned and all services sections", () => {
    render(
      <ServicesAccordion
        pinnedServices={services.filter((service) => service.pinned)}
        services={services}
        onTogglePin={vi.fn()}
      />
    );

    expect(screen.getByText("Pinned services")).toBeInTheDocument();
    expect(screen.getByText("All services")).toBeInTheDocument();
    expect(
      screen.getByText("Quick access to your most-used tools")
    ).toBeInTheDocument();
  });

  it("renders pinned services in the pinned section by default", () => {
    render(
      <ServicesAccordion
        pinnedServices={services.filter((service) => service.pinned)}
        services={services}
        onTogglePin={vi.fn()}
      />
    );

    expect(screen.getAllByText("JupyterHub")).toHaveLength(2);
  });

  it("renders all services in the services section by default", () => {
    render(
      <ServicesAccordion
        pinnedServices={services.filter((service) => service.pinned)}
        services={services}
        onTogglePin={vi.fn()}
      />
    );

    expect(screen.getAllByText("JupyterHub")).toHaveLength(2);
    expect(screen.getByText("Grafana")).toBeInTheDocument();
  });
});
