import { renderWithProviders as render } from "@/test/render";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Service } from "@/api/listServices";
import { ServiceDetailsDrawer } from "@/components/ServiceDetailsDrawer";

const argo: Service = {
  id: "argo",
  name: "Argo CD",
  status: "Healthy",
  description: "Keeps apps in sync with Git.\n\nRoll back when a deployment goes wrong.",
  category: ["Platform"],
  pinned: false,
  url: "https://argo.example.com",
  version: "v2.13.0",
  docsUrl: "https://argo-cd.readthedocs.io/en/stable/",
};

describe("ServiceDetailsDrawer", () => {
  it("shows the name, full description, and metadata", async () => {
    render(<ServiceDetailsDrawer service={argo} open onOpenChange={vi.fn()} />);

    const drawer = await screen.findByRole("dialog", { name: "Argo CD" });
    expect(drawer).toHaveAccessibleDescription(/Keeps apps in sync with Git/);
    expect(within(drawer).getByText(/Roll back when a deployment goes wrong/)).toBeInTheDocument();

    const terms = within(drawer)
      .getAllByRole("term")
      .map((term) => term.textContent);
    expect(terms).toEqual(["Category", "Version", "Docs"]);
    expect(within(drawer).getByText("Platform")).toBeInTheDocument();
    expect(within(drawer).getByText("v2.13.0")).toBeInTheDocument();

    const docs = within(drawer).getByRole("link", { name: /argo-cd\.readthedocs\.io/ });
    expect(docs).toHaveAttribute("href", "https://argo-cd.readthedocs.io/en/stable/");
    expect(docs).toHaveAttribute("target", "_blank");
  });

  it("leaves out metadata rows the service doesn't have", async () => {
    render(
      <ServiceDetailsDrawer
        service={{ ...argo, version: undefined, docsUrl: undefined }}
        open
        onOpenChange={vi.fn()}
      />,
    );

    const drawer = await screen.findByRole("dialog", { name: "Argo CD" });
    expect(
      within(drawer)
        .getAllByRole("term")
        .map((term) => term.textContent),
    ).toEqual(["Category"]);
    expect(within(drawer).queryByRole("link")).not.toBeInTheDocument();
  });

  it("focuses the close button and asks to close from it or Escape", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    render(<ServiceDetailsDrawer service={argo} open onOpenChange={onOpenChange} />);

    const close = await screen.findByRole("button", { name: "Close" });
    await vi.waitFor(() => expect(close).toHaveFocus());

    await user.click(close);
    expect(onOpenChange).toHaveBeenLastCalledWith(false, expect.anything());

    onOpenChange.mockClear();
    await user.keyboard("{Escape}");
    expect(onOpenChange).toHaveBeenLastCalledWith(false, expect.anything());
  });
});
