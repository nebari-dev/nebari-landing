import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StatusBadge } from "@/components/StatusBadge";

describe("StatusBadge", () => {
  it("renders healthy", () => {
    render(<StatusBadge status="Healthy" />);
    expect(screen.getByText("Healthy")).toBeInTheDocument();
  });

  it("renders unhealthy and normalizes casing", () => {
    render(<StatusBadge status="unhealthy" />);
    expect(screen.getByText("Unhealthy")).toBeInTheDocument();
  });

  it("capitalizes unknown values", () => {
    render(<StatusBadge status="unknown" />);
    expect(screen.getByText("Unknown")).toBeInTheDocument();
  });
});
