import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ServiceGridCard } from "@/components/ServiceGridCard";

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
    render(
      <ServiceGridCard service={baseService} onTogglePin={vi.fn()} />
    );

    expect(screen.getByText("JupyterHub")).toBeInTheDocument();
    expect(screen.getByText("Notebook platform")).toBeInTheDocument();
    expect(screen.getByText("data")).toBeInTheDocument();
  });

  it("renders the outer card link", () => {
    render(
      <ServiceGridCard service={baseService} onTogglePin={vi.fn()} />
    );

    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("href", "https://example.com/jupyterhub");
    expect(link).toHaveAttribute("target", "_blank");
  });

  it("calls onTogglePin with the next pinned state", async () => {
    const user = userEvent.setup();
    const onTogglePin = vi.fn();

    render(
      <ServiceGridCard
        service={baseService}
        onTogglePin={onTogglePin}
      />
    );

    await user.click(screen.getByRole("button", { name: /pin service/i }));

    expect(onTogglePin).toHaveBeenCalledWith("svc-1", true);
  });

  it("calls onTogglePin with false when the service is already pinned", async () => {
    const user = userEvent.setup();
    const onTogglePin = vi.fn();

    render(
      <ServiceGridCard
        service={{ ...baseService, pinned: true }}
        onTogglePin={onTogglePin}
      />
    );

    await user.click(screen.getByRole("button", { name: /unpin service/i }));

    expect(onTogglePin).toHaveBeenCalledWith("svc-1", false);
  });
});
