import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Banner } from "@/components/Banner";

describe("Banner", () => {
  it("renders the configured text", () => {
    render(<Banner position="top" config={{ text: "CUI" }} />);
    expect(screen.getByText("CUI")).toBeInTheDocument();
  });

  it("renders nothing when config is absent", () => {
    const { container } = render(<Banner position="top" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when text is empty", () => {
    const { container } = render(<Banner position="top" config={{ text: "" }} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("pins to the edge matching its position", () => {
    render(<Banner position="top" config={{ text: "TOP" }} />);
    render(<Banner position="bottom" config={{ text: "BOTTOM" }} />);
    expect(screen.getByText("TOP")).toHaveClass("fixed", "top-0");
    expect(screen.getByText("BOTTOM")).toHaveClass("fixed", "bottom-0");
  });

  it("publishes its height as a CSS variable and removes it on unmount", () => {
    const { unmount } = render(<Banner position="bottom" config={{ text: "CUI" }} />);

    expect(document.documentElement.style.getPropertyValue("--bottom-banner-height")).toBe(
      "0px", // jsdom reports offsetHeight 0
    );

    unmount();

    expect(document.documentElement.style.getPropertyValue("--bottom-banner-height")).toBe("");
  });

  it("applies configured background and foreground colors", () => {
    render(
      <Banner position="top" config={{ text: "CUI", background: "#502b85", foreground: "#ffffff" }} />,
    );
    const banner = screen.getByText("CUI");
    expect(banner).toHaveStyle({ backgroundColor: "#502b85", color: "#ffffff" });
  });

  it("ignores unsafe color values", () => {
    render(
      <Banner
        position="top"
        config={{
          text: "CUI",
          background: 'red; background-image: url("https://evil.example/x")',
          foreground: "expression(alert(1))",
        }}
      />,
    );
    const banner = screen.getByText("CUI");
    expect(banner.style.backgroundColor).toBe("");
    expect(banner.style.color).toBe("");
  });

  it("treats banner text as plain text, not HTML", () => {
    render(<Banner position="top" config={{ text: "<b>CUI</b>" }} />);
    const banner = screen.getByText("<b>CUI</b>");
    expect(banner.querySelector("b")).toBeNull();
  });
});
