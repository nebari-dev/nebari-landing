import { renderWithProviders as render } from "@/test/render";
import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ServiceIcon } from "@/components/ServiceIcon";

describe("ServiceIcon", () => {
  it("renders the provided image", () => {
    const { container } = render(
      <ServiceIcon name="JupyterHub" image="/images/test-service.svg" />,
    );

    const image = container.querySelector("img");
    expect(image).toBeInTheDocument();
    expect(image).toHaveAttribute("src", "/images/test-service.svg");
  });

  it("renders initials when no image is provided", () => {
    const { container } = render(<ServiceIcon name="Dask Gateway" />);

    expect(container.querySelector("img")).not.toBeInTheDocument();
    expect(screen.getByText("DG")).toBeInTheDocument();
  });

  it("uses the first two letters of a single-word name", () => {
    render(<ServiceIcon name="keycloak" />);

    expect(screen.getByText("KE")).toBeInTheDocument();
  });

  it("falls back to initials when the image fails to load", () => {
    const { container } = render(<ServiceIcon name="Grafana" image="/broken.svg" />);

    const image = container.querySelector("img");
    if (!image) throw new Error("Expected an image before the load error");
    fireEvent.error(image);

    expect(container.querySelector("img")).not.toBeInTheDocument();
    expect(screen.getByText("GR")).toBeInTheDocument();
  });
});
