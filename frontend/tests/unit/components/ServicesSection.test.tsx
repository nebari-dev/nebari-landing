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
    priority: 100,
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
    priority: 100,
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
    priority: 100,
  },
];

const chipLabels = () =>
  screen.queryAllByRole("button", { name: /^Remove / }).map((b) => b.getAttribute("aria-label"));

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
      "Sort: Default",
    );
    expect(screen.getByRole("group", { name: "View" })).toBeInTheDocument();
  });

  it("filters both sections by query", async () => {
    const user = userEvent.setup();
    render(<ServicesSection services={services} onTogglePin={vi.fn()} />);

    await user.type(screen.getByPlaceholderText(/search/i), "jupyter");

    expect(screen.getByText("JupyterHub")).toBeInTheDocument();
    expect(screen.queryByText("Grafana")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Pinned/ })).not.toBeInTheDocument();
  });

  it("keeps category chips in menu order, collapses extras into +N, and reads all as All categories", async () => {
    const user = userEvent.setup();
    render(<ServicesSection services={services} onTogglePin={vi.fn()} />);

    const filter = screen.getByRole("combobox", { name: "Filter by category" });
    await user.click(filter);
    await user.click(await screen.findByRole("option", { name: "platform" }));
    await user.click(screen.getByRole("option", { name: "data" }));

    expect(chipLabels()).toEqual(["Remove data", "Remove platform"]);

    await user.click(screen.getByRole("option", { name: "observability" }));

    expect(screen.queryAllByRole("button", { name: /^Remove / })).toHaveLength(0);
    expect(screen.getByText("All categories")).toBeInTheDocument();
  });

  it("keeps uncategorized services visible when every category is selected", async () => {
    const user = userEvent.setup();
    const withUncategorized = [
      ...services,
      { ...services[0], id: "4", name: "Docs Portal", category: [], pinned: false },
    ];
    render(<ServicesSection services={withUncategorized} onTogglePin={vi.fn()} />);

    await user.click(screen.getByRole("combobox", { name: "Filter by category" }));
    for (const name of ["data", "observability", "platform"]) {
      await user.click(await screen.findByRole("option", { name }));
    }

    expect(screen.getByText("Docs Portal")).toBeInTheDocument();
  });

  it("removes the chip that was clicked, whatever order categories were picked in", async () => {
    const user = userEvent.setup();
    render(<ServicesSection services={services} onTogglePin={vi.fn()} />);

    await user.click(screen.getByRole("combobox", { name: "Filter by category" }));
    await user.click(await screen.findByRole("option", { name: "platform" }));
    await user.click(screen.getByRole("option", { name: "data" }));
    await user.keyboard("{Escape}");

    await user.click(screen.getByRole("button", { name: "Remove data" }));

    expect(chipLabels()).toEqual(["Remove platform"]);
    expect(screen.getByText("Argo CD")).toBeInTheDocument();
    expect(screen.queryByText("JupyterHub")).not.toBeInTheDocument();
  });

  it("removes the last visible chip on Backspace, not one hidden in +N", async () => {
    const user = userEvent.setup();
    const withFourCategories = [
      ...services,
      { ...services[0], id: "4", name: "Docs Portal", category: ["zdocs"] },
    ];
    render(<ServicesSection services={withFourCategories} onTogglePin={vi.fn()} />);

    const filter = screen.getByRole("combobox", { name: "Filter by category" });
    await user.click(filter);
    for (const name of ["platform", "data", "observability"]) {
      await user.click(await screen.findByRole("option", { name }));
    }
    expect(chipLabels()).toEqual(["Remove data", "Remove observability"]);
    expect(screen.getByText("+1", { exact: false })).toBeInTheDocument();

    await user.click(filter);
    await user.keyboard("{Backspace}");

    expect(chipLabels()).toEqual(["Remove data", "Remove platform"]);
    expect(screen.queryByText("+1", { exact: false })).not.toBeInTheDocument();
  });

  it("ignores a selected category while it has no services, and restores it when they return", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<ServicesSection services={services} onTogglePin={vi.fn()} />);

    await user.click(screen.getByRole("combobox", { name: "Filter by category" }));
    await user.click(await screen.findByRole("option", { name: "platform" }));
    await user.keyboard("{Escape}");
    expect(screen.queryByText("JupyterHub")).not.toBeInTheDocument();

    const withoutArgo = services.filter((service) => service.id !== "3");
    rerender(<ServicesSection services={withoutArgo} onTogglePin={vi.fn()} />);

    expect(chipLabels()).toEqual([]);
    expect(screen.queryByText("No services match the selected categories")).not.toBeInTheDocument();
    expect(screen.getByText("JupyterHub")).toBeInTheDocument();
    expect(screen.getByText("Grafana")).toBeInTheDocument();

    rerender(<ServicesSection services={services} onTogglePin={vi.fn()} />);

    expect(chipLabels()).toEqual(["Remove platform"]);
    expect(screen.getByText("Argo CD")).toBeInTheDocument();
    expect(screen.queryByText("JupyterHub")).not.toBeInTheDocument();
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

  it("explains a search with no results and clears it", async () => {
    const user = userEvent.setup();
    render(<ServicesSection services={services} onTogglePin={vi.fn()} />);

    await user.type(screen.getByRole("textbox", { name: "Search services" }), "kubeflow");

    expect(screen.getByRole("status")).toHaveTextContent('No services match "kubeflow"');
    expect(screen.queryByRole("button", { name: /^Pinned/ })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Clear search" }));

    expect(screen.getByRole("textbox", { name: "Search services" })).toHaveValue("");
    expect(screen.getByRole("button", { name: "All services 2" })).toBeInTheDocument();
  });

  it("shows the no-results state in the table view too", async () => {
    const user = userEvent.setup();
    render(<ServicesSection services={services} onTogglePin={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Table" }));
    await user.type(screen.getByRole("textbox", { name: "Search services" }), "kubeflow");

    expect(screen.queryByRole("table", { name: "Services" })).not.toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent('No services match "kubeflow"');
  });

  it("remembers the Grid / Table choice", async () => {
    const user = userEvent.setup();
    const { unmount } = render(<ServicesSection services={services} onTogglePin={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Table" }));
    unmount();
    render(<ServicesSection services={services} onTogglePin={vi.fn()} />);

    expect(screen.getByRole("button", { name: "Table" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("table", { name: "Services" })).toBeInTheDocument();
  });

  it("shows skeletons instead of the no-results state while loading", () => {
    render(<ServicesSection services={[]} onTogglePin={vi.fn()} loading />);

    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Pinned" })).toBeInTheDocument();
    expect(document.querySelectorAll('[data-slot="card"][aria-hidden="true"]').length).toBeGreaterThan(
      0,
    );
  });
});
