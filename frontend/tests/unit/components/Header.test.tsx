import { renderWithProviders as render } from "@/test/render";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { Header } from "@/components/Header";

describe("Header", () => {
  it("shows sign in button when no user is present", () => {
    render(<Header />);
    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
  });

  it("shows user name when signed in", () => {
    render(<Header user={{ name: "John Doe", email: "john@example.com" }} />);

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
      />,
    );

    await user.click(screen.getByRole("button", { name: /account menu/i }));
    await user.click(await screen.findByRole("menuitemradio", { name: /dark mode/i }));
    expect(onThemeChange).toHaveBeenCalledWith("dark");

    await user.click(screen.getByRole("menuitemradio", { name: /light mode/i }));
    expect(onThemeChange).toHaveBeenCalledWith("light");

    await user.click(screen.getByRole("menuitemradio", { name: /system theme/i }));
    expect(onThemeChange).toHaveBeenCalledWith("system");
  });

  it("reflects the current theme mode via aria-checked", async () => {
    const user = userEvent.setup();

    render(<Header user={{ name: "John Doe" }} themeMode="dark" />);

    await user.click(screen.getByRole("button", { name: /account menu/i }));

    expect(await screen.findByRole("menuitemradio", { name: /dark mode/i })).toHaveAttribute(
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

  it("calls onSignOut from the account menu", async () => {
    const user = userEvent.setup();
    const onSignOut = vi.fn();

    render(<Header user={{ name: "John Doe" }} onSignOut={onSignOut} />);

    await user.click(screen.getByRole("button", { name: /account menu/i }));
    await user.click(await screen.findByRole("menuitem", { name: /sign out/i }));

    expect(onSignOut).toHaveBeenCalledOnce();
  });
});

describe("Header admin entry point", () => {
  const user = { name: "Alice Alvarez", email: "alice@example.com" };

  it("hides the admin link and menu item for non-admins", async () => {
    render(<Header user={user} />);
    expect(screen.queryByRole("link", { name: "Admin" })).not.toBeInTheDocument();

    await userEvent.setup().click(screen.getByRole("button", { name: /account menu/i }));
    await screen.findByRole("menuitem", { name: /sign out/i });
    expect(screen.queryByRole("menuitem", { name: /administration/i })).not.toBeInTheDocument();
  });

  it("shows the admin link and marks it current inside the admin area", async () => {
    render(<Header user={user} isAdmin adminActive />, { initialEntries: ["/admin/users"] });
    const link = screen.getByRole("link", { name: "Admin" });
    expect(link).toHaveAttribute("href", "/admin");
    expect(link).toHaveAttribute("aria-current", "page");

    await userEvent.setup().click(screen.getByRole("button", { name: /account menu/i }));
    expect(await screen.findByRole("menuitem", { name: /administration/i })).toHaveAttribute(
      "href",
      "/admin",
    );
  });
});
