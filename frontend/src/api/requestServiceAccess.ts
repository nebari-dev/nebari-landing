import { apiFetch } from "./client";

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
  const { userId, serviceId, timestamp } = payload;

  if (!userId) throw new Error("userId is required");
  if (!serviceId) throw new Error("serviceId is required");
  if (!timestamp) throw new Error("timestamp is required");

  const resp = await apiFetch(`/services/${serviceId}/request_access`, {
    method: "POST",
    body: JSON.stringify({ userId, serviceId, timestamp }),
  });

  if (!resp.ok) {
    throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  }

  return resp.json();
}
