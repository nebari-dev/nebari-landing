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

  it("uses the light service icon variant in light mode", () => {
    const { container } = render(
      <ServiceGridCard
        service={{
          ...baseService,
          image: "/images/default-service.svg",
          iconLight: "/images/light-service.svg",
          iconDark: "/images/dark-service.svg",
        }}
        onTogglePin={vi.fn()}
      />
    );

    expect(container.querySelector("img")).toHaveAttribute(
      "src",
      "/images/light-service.svg"
    );
  });

  it("uses the dark service icon variant in dark mode", () => {
    const { container } = render(
      <ServiceGridCard
        service={{
          ...baseService,
          image: "/images/default-service.svg",
          iconLight: "/images/light-service.svg",
          iconDark: "/images/dark-service.svg",
        }}
        isDarkMode
        onTogglePin={vi.fn()}
      />
    );

    expect(container.querySelector("img")).toHaveAttribute(
      "src",
      "/images/dark-service.svg"
    );
  });

  it("falls back to the default service image when a theme variant is missing", () => {
    const { container } = render(
      <ServiceGridCard
        service={{
          ...baseService,
          image: "/images/default-service.svg",
          iconLight: "/images/light-service.svg",
        }}
        isDarkMode
        onTogglePin={vi.fn()}
      />
    );

    expect(container.querySelector("img")).toHaveAttribute(
      "src",
      "/images/default-service.svg"
    );
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
