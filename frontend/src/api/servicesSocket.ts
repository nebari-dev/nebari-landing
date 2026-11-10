import type { HealthState } from "./listServices";
import { createWebSocketClient } from "./ws";

export type BackendSocketService = {
  uid: string;
  name: string;
  namespace: string;
  displayName: string;
  description: string;
  url: string;
  icon?: string;
  iconLight?: string;
  iconDark?: string;
  category: string;
  version?: string;
  docsUrl?: string;
  priority: number;
  visibility: string;
  health: {
    status: string;
    state?: HealthState;
    lastCheck: string;
    message?: string;
  };
};

export type ServiceSocketMessage = {
  type: "added" | "modified" | "deleted";
  service: BackendSocketService;
};

type ServiceSocketHandlers = {
  onMessage: (message: ServiceSocketMessage) => void;
  onOpen?: () => void;
  onClose?: () => void;
  onError?: (event: Event) => void;
};

export function createServicesSocket(handlers: ServiceSocketHandlers) {
  return createWebSocketClient<ServiceSocketMessage>({
    path: "/ws",
    onMessage: handlers.onMessage,
    onOpen: handlers.onOpen,
    onClose: handlers.onClose,
    onError: handlers.onError,
  });
}
