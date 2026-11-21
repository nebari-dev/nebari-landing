import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { healthState, isUnavailable, StatusBadge } from "@/components/StatusBadge";

describe("StatusBadge for admins", () => {
  it("renders Degraded for a degraded service", () => {
    render(<StatusBadge view="admin" status="unhealthy" health="degraded" />);
    expect(screen.getByText("Degraded")).toBeInTheDocument();
  });

  it("renders Down for a down service", () => {
    render(<StatusBadge view="admin" status="unhealthy" health="down" />);
    expect(screen.getByText("Down")).toBeInTheDocument();
  });

  it("falls back to the legacy status when health is absent", () => {
    render(<StatusBadge view="admin" status="Unhealthy" />);
    expect(screen.getByText("Down")).toBeInTheDocument();
  });

  it("prefers the health badge over running sessions", () => {
    const { rerender } = render(
      <StatusBadge view="admin" status="unhealthy" health="down" sessionCount={3} />,
    );
    expect(screen.getByText("Down")).toBeInTheDocument();
    expect(screen.queryByText(/running/)).not.toBeInTheDocument();

    rerender(<StatusBadge view="admin" status="unhealthy" health="degraded" sessionCount={3} />);
    expect(screen.getByText("Degraded")).toBeInTheDocument();
    expect(screen.queryByText(/running/)).not.toBeInTheDocument();
  });
});

describe("StatusBadge for users", () => {
  it("renders Unavailable for a down service", () => {
    render(<StatusBadge view="user" status="unhealthy" health="down" />);
    expect(screen.getByText("Unavailable")).toBeInTheDocument();
    expect(screen.queryByText("Down")).not.toBeInTheDocument();
  });

  it("does not reveal a degraded state", () => {
    const { container } = render(<StatusBadge view="user" status="unhealthy" health="degraded" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows running sessions for a degraded service", () => {
    render(<StatusBadge view="user" status="unhealthy" health="degraded" sessionCount={1} />);
    expect(screen.getByText("1 session running")).toBeInTheDocument();
  });

  it("prefers Unavailable over running sessions", () => {
    render(<StatusBadge view="user" status="unhealthy" health="down" sessionCount={3} />);
    expect(screen.getByText("Unavailable")).toBeInTheDocument();
    expect(screen.queryByText(/running/)).not.toBeInTheDocument();
  });
});

describe.each(["admin", "user"] as const)("StatusBadge (%s)", (view) => {
  it("renders nothing for a healthy or unknown service", () => {
    const { container, rerender } = render(
      <StatusBadge view={view} status="healthy" health="healthy" />,
    );
    expect(container).toBeEmptyDOMElement();

    rerender(<StatusBadge view={view} status="unknown" health="unknown" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows running sessions for an available service", () => {
    const { rerender } = render(
      <StatusBadge view={view} status="healthy" health="healthy" sessionCount={2} />,
    );
    expect(screen.getByText("2 sessions running")).toBeInTheDocument();

    rerender(<StatusBadge view={view} status="unknown" sessionCount={1} />);
    expect(screen.getByText("1 session running")).toBeInTheDocument();
  });

  it("renders nothing for zero sessions", () => {
    const { container } = render(
      <StatusBadge view={view} status="healthy" health="healthy" sessionCount={0} />,
    );
    expect(container).toBeEmptyDOMElement();
  });
});

describe("healthState", () => {
  it("prefers the health field over the legacy status", () => {
    expect(healthState({ status: "unhealthy", health: "degraded" })).toBe("degraded");
  });

  it("maps the legacy status when health is absent", () => {
    expect(healthState({ status: "Healthy" })).toBe("healthy");
    expect(healthState({ status: "Unhealthy" })).toBe("down");
    expect(healthState({ status: "Unknown" })).toBe("unknown");
  });
});

describe("isUnavailable", () => {
  it("is true only for a down service in the user view", () => {
    expect(isUnavailable({ status: "unhealthy", health: "down" }, "user")).toBe(true);
    expect(isUnavailable({ status: "Unhealthy" }, "user")).toBe(true);
    expect(isUnavailable({ status: "unhealthy", health: "degraded" }, "user")).toBe(false);
    expect(isUnavailable({ status: "healthy", health: "healthy" }, "user")).toBe(false);
  });

  it("is never true in the admin view", () => {
    expect(isUnavailable({ status: "unhealthy", health: "down" }, "admin")).toBe(false);
  });
});
