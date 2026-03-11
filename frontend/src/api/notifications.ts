import { apiFetch } from "./client";

export type Notification = {
  id: string;
  image: string;
  title: string;
  message: string;
  read: boolean;
  createdAt: string;
};

/**
 * Fetches notifications from the webapi.
 */
export async function listNotifications(): Promise<Notification[]> {
  const resp = await apiFetch("/notifications");

  if (!resp.ok) {
    throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  }

  const json = await resp.json();
  return Array.isArray(json) ? json : (json.notifications ?? []);
}

/**
 * Marks a notification as read.
 */
export async function markNotificationRead(notificationId: string): Promise<void> {
  if (!notificationId) {
    throw new Error("notificationId is required");
  }

  const resp = await apiFetch(`/notifications/${notificationId}/read`, {
    method: "PUT",
  });

  if (!resp.ok) {
    throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  }
}
