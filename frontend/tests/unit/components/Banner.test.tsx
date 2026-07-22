import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Banner } from "@/components/Banner";

describe("Banner", () => {
  it("renders the configured text", () => {
    render(<Banner config={{ text: "CUI" }} />);
    expect(screen.getByText("CUI")).toBeInTheDocument();
  });

  it("renders nothing when config is absent", () => {
    const { container } = render(<Banner />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when text is empty", () => {
    const { container } = render(<Banner config={{ text: "" }} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("applies configured background and foreground colors", () => {
    render(<Banner config={{ text: "CUI", background: "#502b85", foreground: "#ffffff" }} />);
    const banner = screen.getByText("CUI");
    expect(banner).toHaveStyle({ backgroundColor: "#502b85", color: "#ffffff" });
  });

  it("ignores unsafe color values", () => {
    render(
      <Banner
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
    render(<Banner config={{ text: "<b>CUI</b>" }} />);
    const banner = screen.getByText("<b>CUI</b>");
    expect(banner.querySelector("b")).toBeNull();
  });
});
