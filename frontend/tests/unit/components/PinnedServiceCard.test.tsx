import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { PinnedServiceCard } from "@/components/PinnedServiceCard";

const service = {
  id: "svc-1",
  name: "JupyterHub",
  status: "Healthy",
  description: "Notebook platform",
  category: ["data"],
  pinned: true,
  image: "",
  url: "https://example.com/jupyterhub",
};

describe("PinnedServiceCard", () => {
  it("renders the service name and status", () => {
    render(<PinnedServiceCard service={service} />);

    expect(screen.getByText("JupyterHub")).toBeInTheDocument();
    expect(screen.getByText("Healthy")).toBeInTheDocument();
  });

  it("renders a link to the service url", () => {
    render(<PinnedServiceCard service={service} />);

    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("href", "https://example.com/jupyterhub");
    expect(link).toHaveAttribute("target", "_blank");
  });
});
