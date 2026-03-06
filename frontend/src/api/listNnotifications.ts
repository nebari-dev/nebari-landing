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

  return resp.json();
}
