import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ServicesTable } from "@/components/ServiceTable";

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

describe("ServicesTable", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders the table headers", () => {
    render(<ServicesTable services={services} onTogglePin={vi.fn()} />);

    expect(screen.getByText("Service")).toBeInTheDocument();
    expect(screen.getByText("Category")).toBeInTheDocument();
    expect(screen.getByText("Status")).toBeInTheDocument();
    expect(screen.getByText("Actions")).toBeInTheDocument();
  });

  it("renders the services and their categories", () => {
    render(<ServicesTable services={services} onTogglePin={vi.fn()} />);

    expect(screen.getByText("JupyterHub")).toBeInTheDocument();
    expect(screen.getByText("Grafana")).toBeInTheDocument();
    expect(screen.getByText("data")).toBeInTheDocument();
    expect(screen.getByText("observability")).toBeInTheDocument();
  });

  it("renders clickable service rows", () => {
    render(<ServicesTable services={services} onTogglePin={vi.fn()} />);

    expect(
      screen.getByRole("link", { name: /jupyterhub \(opens in a new tab\)/i })
    ).toBeInTheDocument();

    expect(
      screen.getByRole("link", { name: /grafana \(opens in a new tab\)/i })
    ).toBeInTheDocument();
  });

  it("opens the service url when a row is clicked", async () => {
    const user = userEvent.setup();
    const openSpy = vi.spyOn(window, "open").mockImplementation(() => null);

    render(<ServicesTable services={services} onTogglePin={vi.fn()} />);

    await user.click(
      screen.getByRole("link", { name: /jupyterhub \(opens in a new tab\)/i })
    );

    expect(openSpy).toHaveBeenCalledWith(
      "https://example.com/jupyterhub",
      "_blank",
      "noopener,noreferrer"
    );
  });

  it("calls onTogglePin when the pin button is clicked", async () => {
    const user = userEvent.setup();
    const onTogglePin = vi.fn();

    render(<ServicesTable services={services} onTogglePin={onTogglePin} />);

    await user.click(screen.getByRole("button", { name: /^Pin service$/i }));

    expect(onTogglePin).toHaveBeenCalledWith("svc-1", true);
  });

  it("calls onTogglePin with false for a pinned service", async () => {
    const user = userEvent.setup();
    const onTogglePin = vi.fn();

    render(<ServicesTable services={services} onTogglePin={onTogglePin} />);

    await user.click(screen.getByRole("button", { name: /^Unpin service$/i }));

    expect(onTogglePin).toHaveBeenCalledWith("svc-2", false);
  });
});
