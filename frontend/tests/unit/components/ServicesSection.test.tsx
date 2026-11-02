import { renderWithProviders as render } from "@/test/render";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ServicesSection } from "@/components/ServicesSection";

const services = [
  {
    id: "1",
    name: "JupyterHub",
    status: "Healthy",
    description: "Notebook platform",
    category: ["data"],
    pinned: false,
    image: "",
    url: "https://example.com/jupyter",
  },
  {
    id: "2",
    name: "Grafana",
    status: "Healthy",
    description: "Metrics dashboards",
    category: ["observability"],
    pinned: true,
    image: "",
    url: "https://example.com/grafana",
  },
  {
    id: "3",
    name: "Argo CD",
    status: "Healthy",
    description: "GitOps deployments",
    category: ["platform"],
    pinned: false,
    image: "",
    url: "https://example.com/argo",
  },
];

const cardNames = (region: HTMLElement) =>
  within(region)
    .getAllByRole("link")
    .map((link) => link.getAttribute("aria-label")?.replace(" (opens in a new tab)", ""));

describe("ServicesSection", () => {
  it("renders the search, filter, sort, and view controls", () => {
    render(<ServicesSection services={services} onTogglePin={vi.fn()} />);

    expect(screen.getByRole("textbox", { name: "Search services" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Filter by category" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Sort services" })).toHaveTextContent(
      "Sort: Name",
    );
    expect(screen.getByRole("group", { name: "View" })).toBeInTheDocument();
  });

  it("filters both sections by query", async () => {
    const user = userEvent.setup();
    render(<ServicesSection services={services} onTogglePin={vi.fn()} />);

    await user.type(screen.getByPlaceholderText(/search/i), "jupyter");

    expect(screen.getByText("JupyterHub")).toBeInTheDocument();
    expect(screen.queryByText("Grafana")).not.toBeInTheDocument();
    expect(screen.getByText("No pinned services yet.")).toBeInTheDocument();
  });

  it("filters by the selected categories", async () => {
    const user = userEvent.setup();
    render(<ServicesSection services={services} onTogglePin={vi.fn()} />);

    await user.click(screen.getByRole("combobox", { name: "Filter by category" }));
    await user.click(await screen.findByRole("option", { name: "platform" }));

    expect(screen.getByText("Argo CD")).toBeInTheDocument();
    expect(screen.queryByText("JupyterHub")).not.toBeInTheDocument();
    expect(screen.queryByText("Grafana")).not.toBeInTheDocument();
  });

  it("shows each service once, with pinned services only in the pinned section", () => {
    render(<ServicesSection services={services} onTogglePin={vi.fn()} />);

    expect(screen.getByRole("button", { name: "Pinned 1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "All services 2" })).toBeInTheDocument();
    expect(cardNames(screen.getByRole("region", { name: /^All services/ }))).toEqual([
      "Argo CD",
      "JupyterHub",
    ]);
  });

  it("sorts the cards from the sort dropdown", async () => {
    const user = userEvent.setup();
    render(<ServicesSection services={services} onTogglePin={vi.fn()} />);

    await user.click(screen.getByRole("combobox", { name: "Sort services" }));
    await user.click(await screen.findByRole("option", { name: "Name (Z–A)" }));

    expect(screen.getByRole("combobox", { name: "Sort services" })).toHaveTextContent(
      "Sort: Name (Z–A)",
    );
    expect(cardNames(screen.getByRole("region", { name: /^All services/ }))).toEqual([
      "JupyterHub",
      "Argo CD",
    ]);
  });

  it("defaults to grid view and switches to table view", async () => {
    const user = userEvent.setup();
    render(<ServicesSection services={services} onTogglePin={vi.fn()} />);

    expect(screen.getByRole("button", { name: "Grid" })).toHaveAttribute("aria-pressed", "true");

    await user.click(screen.getByRole("button", { name: "Table" }));

    expect(screen.getByRole("button", { name: "Table" })).toHaveAttribute("aria-pressed", "true");
    const table = screen.getByRole("table", { name: "Services" });
    expect(within(table).getByText("Grafana")).toBeInTheDocument();
  });

  it("updates the sort dropdown when a table header is clicked", async () => {
    const user = userEvent.setup();
    render(<ServicesSection services={services} onTogglePin={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Table" }));
    await user.click(screen.getByRole("button", { name: "Category" }));

    expect(screen.getByRole("combobox", { name: "Sort services" })).toHaveTextContent(
      "Sort: Category",
    );
    expect(screen.getByRole("columnheader", { name: "Category" })).toHaveAttribute(
      "aria-sort",
      "ascending",
    );
  });
});
