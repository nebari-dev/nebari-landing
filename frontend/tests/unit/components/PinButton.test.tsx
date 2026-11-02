import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { PinButton } from "@/components/PinButton";

describe("PinButton", () => {
  it("offers to pin an unpinned service", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    render(<PinButton pinned={false} onToggle={onToggle} />);

    await user.click(screen.getByRole("button", { name: "Pin service" }));

    expect(onToggle).toHaveBeenCalledWith(true);
  });

  it("offers to unpin a pinned service", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    render(<PinButton pinned onToggle={onToggle} />);

    await user.click(screen.getByRole("button", { name: "Unpin service" }));

    expect(onToggle).toHaveBeenCalledWith(false);
  });
});
