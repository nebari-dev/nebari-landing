import { createWebSocketClient } from "./ws";
// import type { Service } from "./listServices";

export type BackendService = {
  uid: string;
  name: string;
  namespace: string;
  displayName: string;
  description: string;
  url: string;
  icon: string;
  category: string;
  priority: number;
  visibility: string;
  health: {
    status: string;
    lastCheck: string;
    message?: string;
  };
};

export type ServiceSocketMessage = {
  type: "added" | "modified" | "deleted";
  service: BackendService;
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
