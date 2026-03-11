import { createWebSocketClient } from "./ws";
import type { Notification } from "./notifications";

export type NotificationSocketMessage =
  | {
      type: "notification.added";
      notification: Notification;
    }
  | {
      type: "notification.modified";
      notification: Notification;
    }
  | {
      type: "notification.deleted";
      id: string;
    };

type NotificationHandlers = {
  onMessage: (message: NotificationSocketMessage) => void;
  onOpen?: () => void;
  onClose?: () => void;
  onError?: (event: Event) => void;
};

export function createNotificationsSocket(handlers: NotificationHandlers) {
  return createWebSocketClient<NotificationSocketMessage>({
    path: "/ws",
    onMessage: handlers.onMessage,
    onOpen: handlers.onOpen,
    onClose: handlers.onClose,
    onError: handlers.onError,
  });
}
