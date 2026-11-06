import { renderWithProviders as render } from "@/test/render";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ServiceGridCard } from "@/components/ServiceGridCard";
import { StatusViewProvider } from "@/hooks/status-view";

const baseService = {
  id: "svc-1",
  name: "JupyterHub",
  status: "Healthy",
  description: "Notebook platform",
  category: ["data"],
  pinned: false,
  image: "",
  url: "https://example.com/jupyterhub",
};

describe("ServiceGridCard", () => {
  it("renders the service name, description, and category", () => {
    render(<ServiceGridCard service={baseService} onTogglePin={vi.fn()} />);

    expect(screen.getByText("JupyterHub")).toBeInTheDocument();
    expect(screen.getByText("Notebook platform")).toBeInTheDocument();
    expect(screen.getByText("data")).toBeInTheDocument();
  });

  it("renders the outer card link", () => {
    render(<ServiceGridCard service={baseService} onTogglePin={vi.fn()} />);

    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("href", "https://example.com/jupyterhub");
    expect(link).toHaveAttribute("target", "_blank");
  });

  it("shows no status badge for a healthy service", () => {
    render(<ServiceGridCard service={baseService} onTogglePin={vi.fn()} />);

    expect(screen.queryByText("Down")).not.toBeInTheDocument();
    expect(screen.queryByText("Healthy")).not.toBeInTheDocument();
  });

  it("shows a down service as unavailable to a user, without the new-tab icon", () => {
    const { container } = render(
      <ServiceGridCard
        service={{ ...baseService, status: "Unhealthy", health: "down" }}
        onTogglePin={vi.fn()}
      />,
    );

    expect(screen.getByText("Unavailable")).toBeInTheDocument();
    expect(container.querySelector("svg.lucide-external-link")).toBeNull();
  });

  it("hides a degraded state from a user", () => {
    render(
      <ServiceGridCard
        service={{ ...baseService, status: "Unhealthy", health: "degraded" }}
        onTogglePin={vi.fn()}
      />,
    );

    expect(screen.queryByText("Degraded")).not.toBeInTheDocument();
  });

  it("shows an admin the health state and keeps the new-tab icon", () => {
    const { container, rerender } = render(
      <StatusViewProvider view="admin">
        <ServiceGridCard
          service={{ ...baseService, status: "Unhealthy", health: "down" }}
          onTogglePin={vi.fn()}
        />
      </StatusViewProvider>,
    );

    expect(screen.getByText("Down")).toBeInTheDocument();
    expect(screen.queryByText("Unavailable")).not.toBeInTheDocument();
    expect(container.querySelector("svg.lucide-external-link")).not.toBeNull();

    rerender(
      <StatusViewProvider view="admin">
        <ServiceGridCard
          service={{ ...baseService, status: "Unhealthy", health: "degraded" }}
          onTogglePin={vi.fn()}
        />
      </StatusViewProvider>,
    );
    expect(screen.getByText("Degraded")).toBeInTheDocument();
  });

  it("calls onTogglePin with the next pinned state", async () => {
    const user = userEvent.setup();
    const onTogglePin = vi.fn();

    render(<ServiceGridCard service={baseService} onTogglePin={onTogglePin} />);

    await user.click(screen.getByRole("button", { name: /pin service/i }));

    expect(onTogglePin).toHaveBeenCalledWith("svc-1", true);
  });

  it("calls onTogglePin with false when the service is already pinned", async () => {
    const user = userEvent.setup();
    const onTogglePin = vi.fn();

    render(
      <ServiceGridCard service={{ ...baseService, pinned: true }} onTogglePin={onTogglePin} />,
    );

    await user.click(screen.getByRole("button", { name: /unpin service/i }));

    expect(onTogglePin).toHaveBeenCalledWith("svc-1", false);
  });

  it("renders without a pin control when pinning is not offered", () => {
    render(<ServiceGridCard service={baseService} />);

    expect(screen.queryByRole("button", { name: /pin service/i })).not.toBeInTheDocument();
  });

  it("shows the first category and counts the rest", () => {
    render(
      <ServiceGridCard
        service={{ ...baseService, category: ["ML", "Monitoring", "Platform"] }}
        onTogglePin={vi.fn()}
      />,
    );

    expect(screen.getByText("ML")).toBeInTheDocument();
    expect(screen.queryByText("Monitoring")).not.toBeInTheDocument();
    expect(screen.getByText("+2")).toBeInTheDocument();
    expect(screen.getByText("2 more categories: Monitoring, Platform")).toBeInTheDocument();
  });

  it("offers a Details action", () => {
    render(<ServiceGridCard service={baseService} onTogglePin={vi.fn()} />);

    expect(screen.getByRole("button", { name: "JupyterHub details" })).toBeInTheDocument();
  });
});
