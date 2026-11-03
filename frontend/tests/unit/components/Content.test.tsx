import { renderWithProviders as render } from "@/test/render";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Service } from "@/api/listServices";
import { Content } from "@/components/Content";
import type { ServicesStatus } from "@/hooks/useLaunchpadData";

function makeServices(count: number): Service[] {
  return Array.from({ length: count }, (_, index) => ({
    id: `svc-${index}`,
    name: `Service ${index}`,
    status: "Healthy",
    description: `Description ${index}`,
    category: ["Platform"],
    pinned: index === 0,
    url: `https://example.com/${index}`,
  }));
}

function renderContent({
  services = [],
  status = "ready",
  onRetry = vi.fn(),
  userName,
}: {
  services?: Service[];
  status?: ServicesStatus;
  onRetry?: () => void;
  userName?: string;
}) {
  return render(
    <Content
      services={services}
      status={status}
      onRetry={onRetry}
      userName={userName}
      onTogglePin={vi.fn()}
    />,
  );
}

const skeletonCards = () => document.querySelectorAll('[data-slot="card"][aria-hidden="true"]');

describe("Content", () => {
  it("renders the simple layout for fewer than 6 services", () => {
    renderContent({ services: makeServices(5), userName: "Sam" });

    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Welcome back, Sam");
    expect(screen.queryByRole("textbox", { name: "Search services" })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "View" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Pinned/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /pin service/i })).not.toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: /opens in a new tab/ })).toHaveLength(5);
  });

  it("keeps the API order in the simple layout", () => {
    renderContent({ services: makeServices(3).reverse() });

    expect(
      screen
        .getAllByRole("link", { name: /opens in a new tab/ })
        .map((link) => link.getAttribute("aria-label")),
    ).toEqual([
      "Service 2 (opens in a new tab)",
      "Service 1 (opens in a new tab)",
      "Service 0 (opens in a new tab)",
    ]);
  });

  it("opens the details drawer for the card whose Details was pressed", async () => {
    const user = userEvent.setup();
    renderContent({ services: makeServices(3) });

    await user.click(screen.getByRole("button", { name: "Service 1 details" }));

    const drawer = await screen.findByRole("dialog", { name: "Service 1" });
    expect(drawer).toHaveAccessibleDescription("Description 1");
  });

  it("renders the richer layout for 6 or more services", () => {
    renderContent({ services: makeServices(6) });

    expect(screen.getByRole("textbox", { name: "Search services" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Filter by category" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Sort services" })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "View" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Pinned 1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "All services 5" })).toBeInTheDocument();
  });

  it("shows the simple skeleton grid while loading on a first visit", () => {
    renderContent({ status: "loading" });

    expect(screen.getByRole("status")).toHaveTextContent("Loading services…");
    expect(skeletonCards()).toHaveLength(5);
    expect(screen.queryByRole("textbox", { name: "Search services" })).not.toBeInTheDocument();
    expect(screen.queryByText("No services yet")).not.toBeInTheDocument();
  });

  it("shows the richer skeleton sections when the services last loaded in the richer layout", () => {
    window.localStorage.setItem("launchpad:servicesLayout", "richer");
    renderContent({ status: "loading" });

    expect(screen.getByRole("textbox", { name: "Search services" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Pinned" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "All services" })).toBeInTheDocument();
    expect(skeletonCards()).toHaveLength(8);
  });

  it("remembers which layout the services rendered in", () => {
    const { unmount } = renderContent({ services: makeServices(6) });
    expect(window.localStorage.getItem("launchpad:servicesLayout")).toBe("richer");
    unmount();

    renderContent({ services: makeServices(2) });
    expect(window.localStorage.getItem("launchpad:servicesLayout")).toBe("simple");
  });

  it("does not remember a layout while loading or after an error", () => {
    renderContent({ status: "loading" });
    renderContent({ status: "error" });

    expect(window.localStorage.getItem("launchpad:servicesLayout")).toBeNull();
  });

  it("explains an empty deployment", () => {
    renderContent({ services: [] });

    expect(screen.getByRole("heading", { name: "No services yet" })).toBeInTheDocument();
    expect(
      screen.getByText(/Your admin hasn't added any services to this deployment/),
    ).toBeInTheDocument();
  });

  it("reports a failed request and retries on request", async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    renderContent({ status: "error", onRetry });

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Couldn't load services");
    expect(screen.queryByText("No services yet")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Try again" }));

    expect(onRetry).toHaveBeenCalledOnce();
  });
});
