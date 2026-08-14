// This module is currently unused but intentionally retained for future notification work.
// Do not remove it.

import type { Notification } from "./notifications";
import { createWebSocketClient } from "./ws";

export type NotificationSocketMessage = {
  type: "notification.created";
  notification: Notification;
};

type NotificationSocketHandlers = {
  onMessage: (message: NotificationSocketMessage) => void;
  onOpen?: () => void;
  onClose?: () => void;
  onError?: (event: Event) => void;
};

export function createNotificationsSocket(handlers: NotificationSocketHandlers) {
  return createWebSocketClient<NotificationSocketMessage>({
    path: "/ws",
    onMessage: handlers.onMessage,
    onOpen: handlers.onOpen,
    onClose: handlers.onClose,
    onError: handlers.onError,
  });
}
