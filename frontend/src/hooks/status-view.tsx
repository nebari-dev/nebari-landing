import { createContext, type ReactNode, useContext } from "react";

/** Admins see each service's health state; users see only whether it is available. */
export type StatusView = "admin" | "user";

const StatusViewContext = createContext<StatusView>("user");

export function StatusViewProvider({ view, children }: { view: StatusView; children: ReactNode }) {
  return <StatusViewContext.Provider value={view}>{children}</StatusViewContext.Provider>;
}

/** The status view for the signed-in user; "user" outside a provider. */
export function useStatusView(): StatusView {
  return useContext(StatusViewContext);
}
