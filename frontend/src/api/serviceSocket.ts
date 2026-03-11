import { createWebSocketClient } from "./ws";
import type { Service } from "./listServices";

export type ServiceSocketMessage =
  | {
      type: "service.created";
      service: Service;
    }
  | {
      type: "service.updated";
      service: Service;
    }
  | {
      type: "service.deleted";
      id: string;
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
