import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ServiceIcon } from "@/components/ServiceIcon";

describe("ServiceIcon", () => {
  it("renders the provided image", () => {
    const { container } = render(
      <ServiceIcon imageUrl="/images/test-service.svg" />
    );

    const image = container.querySelector("img");
    expect(image).toBeInTheDocument();
    expect(image).toHaveAttribute("src", "/images/test-service.svg");
  });

  it("updates the rendered image when imageUrl changes", () => {
    const { container, rerender } = render(
      <ServiceIcon imageUrl="/images/light-service.svg" />
    );

    expect(container.querySelector("img")).toHaveAttribute(
      "src",
      "/images/light-service.svg"
    );

    rerender(<ServiceIcon imageUrl="/images/dark-service.svg" />);

    expect(container.querySelector("img")).toHaveAttribute(
      "src",
      "/images/dark-service.svg"
    );
  });

  it("renders the fallback image when no imageUrl is provided", () => {
    const { container } = render(<ServiceIcon />);

    const image = container.querySelector("img");
    expect(image).toBeInTheDocument();
    expect(image?.getAttribute("src")).toBeTruthy();
  });
});
