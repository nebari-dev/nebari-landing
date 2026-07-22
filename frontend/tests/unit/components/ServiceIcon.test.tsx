import { renderWithProviders as render } from "@/test/render";
import { describe, expect, it } from "vitest";
import { ServiceIcon } from "@/components/ServiceIcon";

describe("ServiceIcon", () => {
  it("renders the provided image", () => {
    const { container } = render(
      <ServiceIcon image="/images/test-service.svg" />
    );

    const image = container.querySelector("img");
    expect(image).toBeInTheDocument();
    expect(image).toHaveAttribute("src", "/images/test-service.svg");
  });

  it("renders the fallback image when no image is provided", () => {
    const { container } = render(<ServiceIcon />);

    const image = container.querySelector("img");
    expect(image).toBeInTheDocument();
    expect(image?.getAttribute("src")).toBeTruthy();
  });
});
