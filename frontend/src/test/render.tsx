import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { type RenderOptions, render } from "@testing-library/react";
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router";
import { ThemeProvider } from "@/hooks/theme-provider";

type ProviderOptions = {
  /** Initial history entries for the in-memory router. */
  initialEntries?: string[];
};

function makeWrapper({ initialEntries = ["/"] }: ProviderOptions) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={initialEntries}>
          <ThemeProvider>{children}</ThemeProvider>
        </MemoryRouter>
      </QueryClientProvider>
    );
  };
}

export function renderWithProviders(
  ui: ReactElement,
  options?: Omit<RenderOptions, "wrapper"> & ProviderOptions,
) {
  const { initialEntries, ...renderOptions } = options ?? {};
  return render(ui, { wrapper: makeWrapper({ initialEntries }), ...renderOptions });
}
