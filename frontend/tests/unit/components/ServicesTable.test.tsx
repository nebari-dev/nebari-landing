import { renderWithProviders as render } from "@/test/render";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ServicesTable } from "@/components/ServiceTable";
import { DEFAULT_SORT, type ServiceSort } from "@/lib/services";

const services = [
  {
    id: "svc-1",
    name: "JupyterHub",
    status: "Healthy",
    description: "Notebook platform",
    category: ["data"],
    pinned: false,
    image: "",
    url: "https://example.com/jupyterhub",
  },
  {
    id: "svc-2",
    name: "Grafana",
    status: "Unhealthy",
    description: "Metrics dashboards",
    category: ["observability"],
    pinned: true,
    image: "",
    url: "https://example.com/grafana",
  },
];

function renderTable({
  sort = DEFAULT_SORT,
  onSortChange = vi.fn(),
  onTogglePin = vi.fn(),
}: {
  sort?: ServiceSort;
  onSortChange?: (sort: ServiceSort) => void;
  onTogglePin?: (serviceId: string, nextPinned: boolean) => void;
} = {}) {
  return render(
    <ServicesTable
      services={services}
      sort={sort}
      onSortChange={onSortChange}
      onTogglePin={onTogglePin}
    />,
  );
}

describe("ServicesTable", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders the table headers", () => {
    renderTable();

    expect(screen.getByRole("columnheader", { name: "Pinned" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Service" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Category" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Status" })).toBeInTheDocument();
  });

  it("renders the services and their categories", () => {
    renderTable();

    expect(screen.getByText("JupyterHub")).toBeInTheDocument();
    expect(screen.getByText("Grafana")).toBeInTheDocument();
    expect(screen.getByText("data")).toBeInTheDocument();
    expect(screen.getByText("observability")).toBeInTheDocument();
  });

  it("lists pinned services first", () => {
    renderTable();

    const [, firstRow, secondRow] = screen.getAllByRole("row");
    expect(firstRow).toHaveTextContent("Grafana");
    expect(secondRow).toHaveTextContent("JupyterHub");
  });

  it("shows each service's description and status", () => {
    renderTable();

    const row = screen.getByRole("row", { name: /Grafana/ });
    expect(within(row).getByText("Metrics dashboards")).toHaveAttribute(
      "title",
      "Metrics dashboards",
    );
    expect(within(row).getByText("Unhealthy")).toBeInTheDocument();
  });

  it("renders skeleton rows under the headers while loading", () => {
    render(
      <ServicesTable services={[]} sort={DEFAULT_SORT} onSortChange={vi.fn()} loading />,
    );

    expect(screen.getByRole("columnheader", { name: "Service" })).toBeInTheDocument();
    const bodyRows = document.querySelectorAll("tbody tr");
    expect(bodyRows).toHaveLength(8);
    for (const row of bodyRows) {
      expect(row).toHaveAttribute("aria-hidden", "true");
    }
  });

  it("links each service name to the service in a new tab", () => {
    renderTable();

    const link = screen.getByRole("link", { name: "JupyterHub (opens in a new tab)" });
    expect(link).toHaveAttribute("href", "https://example.com/jupyterhub");
    expect(link).toHaveAttribute("target", "_blank");
    expect(screen.queryByRole("link", { name: /^open /i })).not.toBeInTheDocument();
  });

  it("opens the service url when a row is clicked", async () => {
    const user = userEvent.setup();
    const openSpy = vi.spyOn(window, "open").mockImplementation(() => null);

    renderTable();

    await user.click(screen.getByText("Notebook platform"));

    expect(openSpy).toHaveBeenCalledWith(
      "https://example.com/jupyterhub",
      "_blank",
      "noopener,noreferrer",
    );
  });

  it("does not open the service a second time when the name link is clicked", async () => {
    const user = userEvent.setup();
    const openSpy = vi.spyOn(window, "open").mockImplementation(() => null);

    renderTable();

    const link = screen.getByRole("link", { name: /JupyterHub/ });
    link.addEventListener("click", (event) => event.preventDefault());
    await user.click(link);

    expect(openSpy).not.toHaveBeenCalled();
  });

  it("calls onTogglePin when the pin button is clicked", async () => {
    const user = userEvent.setup();
    const onTogglePin = vi.fn();
    const openSpy = vi.spyOn(window, "open").mockImplementation(() => null);

    renderTable({ onTogglePin });

    await user.click(screen.getByRole("button", { name: /^Pin service$/i }));

    expect(onTogglePin).toHaveBeenCalledWith("svc-1", true);
    expect(openSpy).not.toHaveBeenCalled();
  });

  it("calls onTogglePin with false for a pinned service", async () => {
    const user = userEvent.setup();
    const onTogglePin = vi.fn();

    renderTable({ onTogglePin });

    await user.click(screen.getByRole("button", { name: /^Unpin service$/i }));

    expect(onTogglePin).toHaveBeenCalledWith("svc-2", false);
  });

  it("marks the active sort column", () => {
    renderTable({ sort: { key: "category", direction: "desc" } });

    expect(screen.getByRole("columnheader", { name: "Category" })).toHaveAttribute(
      "aria-sort",
      "descending",
    );
    expect(screen.getByRole("columnheader", { name: "Service" })).toHaveAttribute(
      "aria-sort",
      "none",
    );
  });

  it("flips the direction when the active column header is clicked", async () => {
    const user = userEvent.setup();
    const onSortChange = vi.fn();

    renderTable({ onSortChange });

    await user.click(screen.getByRole("button", { name: "Service" }));

    expect(onSortChange).toHaveBeenCalledWith({ key: "name", direction: "desc" });
  });

  it("sorts ascending by a newly chosen column", async () => {
    const user = userEvent.setup();
    const onSortChange = vi.fn();

    renderTable({ sort: { key: "name", direction: "desc" }, onSortChange });

    await user.click(screen.getByRole("button", { name: "Status" }));

    expect(onSortChange).toHaveBeenCalledWith({ key: "status", direction: "asc" });
  });
});
