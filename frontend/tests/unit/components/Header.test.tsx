import { renderWithProviders as render } from "@/test/render";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { Header } from "@/components/Header";

describe("Header", () => {
  it("shows sign in button when no user is present", () => {
    render(<Header notifications={[]} />);
    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
  });

  it("shows user name when signed in", () => {
    render(<Header user={{ name: "John Doe", email: "john@example.com" }} notifications={[]} />);

    expect(screen.getByText("John Doe")).toBeInTheDocument();
  });

  it("selects a theme mode from the profile menu", async () => {
    const user = userEvent.setup();
    const onThemeChange = vi.fn();

    render(
      <Header
        user={{ name: "John Doe" }}
        themeMode="system"
        onThemeChange={onThemeChange}
        notifications={[]}
      />,
    );

    await user.click(screen.getByRole("button", { name: /account menu/i }));
    await user.click(screen.getByRole("menuitemradio", { name: /dark mode/i }));
    expect(onThemeChange).toHaveBeenCalledWith("dark");

    await user.click(screen.getByRole("menuitemradio", { name: /light mode/i }));
    expect(onThemeChange).toHaveBeenCalledWith("light");

    await user.click(screen.getByRole("menuitemradio", { name: /system theme/i }));
    expect(onThemeChange).toHaveBeenCalledWith("system");
  });

  it("reflects the current theme mode via aria-checked", async () => {
    const user = userEvent.setup();

    render(<Header user={{ name: "John Doe" }} themeMode="dark" notifications={[]} />);

    await user.click(screen.getByRole("button", { name: /account menu/i }));

    expect(screen.getByRole("menuitemradio", { name: /dark mode/i })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(screen.getByRole("menuitemradio", { name: /light mode/i })).toHaveAttribute(
      "aria-checked",
      "false",
    );
    expect(screen.getByRole("menuitemradio", { name: /system theme/i })).toHaveAttribute(
      "aria-checked",
      "false",
    );
  });
});
