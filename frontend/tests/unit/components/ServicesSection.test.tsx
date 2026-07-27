import { renderWithProviders as render } from "@/test/render";
import { screen } from "@testing-library/react";
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
];

describe("ServicesSection", () => {
  it("renders search input", () => {
    render(<ServicesSection services={services} onTogglePin={vi.fn()} />);
    expect(screen.getByPlaceholderText(/search/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^search$/i })).not.toBeInTheDocument();
  });

  it("filters services by query", async () => {
    const user = userEvent.setup();
    render(<ServicesSection services={services} onTogglePin={vi.fn()} />);

    await user.type(screen.getByPlaceholderText(/search/i), "grafana");

    expect(screen.getByText("Grafana")).toBeInTheDocument();
    expect(screen.queryByText("JupyterHub")).not.toBeInTheDocument();
  });

  it("defaults to grid view and switches to list view", async () => {
    const user = userEvent.setup();
    render(<ServicesSection services={services} onTogglePin={vi.fn()} />);

    expect(screen.getByRole("tab", { name: /grid view/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    await user.click(screen.getByRole("tab", { name: /list view/i }));

    expect(screen.getByText("Grafana")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /list view/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });
});
