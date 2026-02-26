export type Notification = {
  id: string;
  image: string;
  title: string;
  message: string;
  read: boolean;
  createdAt: string;
};

/**
 * Fetches notifications from the backend
 * @returns list of notifications
 */
export async function listNotifications(): Promise<Notification[]> {

  // get the API_BASE_URL from the environment file
  const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? "").replace(/\/$/, "");
  
  // Guard against API url missing
  if (!API_BASE_URL) {
    throw new Error("VITE_API_BASE_URL is not set");
  }

  // Fetch the resource
  const resp = await fetch(`${API_BASE_URL}/notifications`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
  });

  // Guard against fetch failure
  if (!resp.ok) {
    throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  }

  // convert rresponse to JSON
  const json = await resp.json()

  // return the data to be displayed
  return json;
}
