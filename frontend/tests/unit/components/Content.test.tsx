import { renderWithProviders as render } from "@/test/render";
import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Service } from "@/api/listServices";
import { Content } from "@/components/Content";

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

describe("Content", () => {
  it("renders the simple layout for fewer than 6 services", () => {
    render(<Content services={makeServices(5)} userName="Sam" onTogglePin={vi.fn()} />);

    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Welcome back, Sam");
    expect(screen.queryByRole("textbox", { name: "Search services" })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "View" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Pinned/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /pin service/i })).not.toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: /opens in a new tab/ })).toHaveLength(5);
  });

  it("keeps the API order in the simple layout", () => {
    const services = makeServices(3).reverse();
    render(<Content services={services} onTogglePin={vi.fn()} />);

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

  it("renders the richer layout for 6 or more services", () => {
    render(<Content services={makeServices(6)} onTogglePin={vi.fn()} />);

    expect(screen.getByRole("textbox", { name: "Search services" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Filter by category" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Sort services" })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "View" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Pinned 1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "All services 5" })).toBeInTheDocument();
  });
});
