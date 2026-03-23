import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { Header } from "@/components/Header";

describe("Header", () => {
  it("shows sign in button when no user is present", () => {
    render(<Header notifications={[]} />);
    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
  });

  it("shows user name when signed in", () => {
    render(
      <Header
        user={{ name: "John Doe", email: "john@example.com" }}
        notifications={[]}
      />
    );

    expect(screen.getByText("John Doe")).toBeInTheDocument();
  });

  it("calls theme toggle when clicked", async () => {
    const user = userEvent.setup();
    const onToggleTheme = vi.fn();

    render(
      <Header
        isDarkMode={false}
        onToggleTheme={onToggleTheme}
        notifications={[]}
      />
    );

    await user.click(screen.getByRole("button", { name: /switch to dark mode/i }));
    expect(onToggleTheme).toHaveBeenCalledTimes(1);
  });
});
