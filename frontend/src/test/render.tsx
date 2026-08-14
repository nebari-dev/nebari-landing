import { type RenderOptions, render } from "@testing-library/react";
import type { ReactElement } from "react";
import { ThemeProvider } from "@/hooks/theme-provider";

function Wrapper({ children }: { children: React.ReactNode }) {
  return <ThemeProvider>{children}</ThemeProvider>;
}

export function renderWithProviders(ui: ReactElement, options?: Omit<RenderOptions, "wrapper">) {
  return render(ui, { wrapper: Wrapper, ...options });
}
