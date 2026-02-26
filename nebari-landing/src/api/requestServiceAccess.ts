
export type RequestServiceAccessPayload = {
  userId: string;
  serviceId: string;
  timestamp: string;
};

export type RequestServiceAccessResponse = {
  success: boolean;
  message?: string;
};

export async function requestServiceAccess(
  payload: RequestServiceAccessPayload
): Promise<RequestServiceAccessResponse> {

  // extract the payload props
  const { userId, serviceId, timestamp } = payload;

  // get the API_BASE_URL from the environment file
  const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? "").replace(/\/$/, "");

  // Guard against API url missing
  if (!API_BASE_URL) {
    throw new Error("VITE_API_BASE_URL is not set");
  }

  // Guards for missing information
  if (!userId) throw new Error("userId is required");
  if (!serviceId) throw new Error("serviceId is required");
  if (!timestamp) throw new Error("timestamp is required");

  // send a request
  const resp = await fetch(`${API_BASE_URL}/requestServiceAccess`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      userId,
      serviceId,
      timestamp,
    }),
  });

  // Guard against fetch failure
  if (!resp.ok) {
    throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  }

  // Convert the response to JSON
  const json = await resp.json()

  // return the data
  return json
}
