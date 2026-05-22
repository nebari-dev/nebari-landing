import { apiFetch } from "./client";
import type { Service } from "./listServices";

export type { Service };

export async function getServiceById(serviceId: string): Promise<Service> {
  if (!serviceId) {
    throw new Error("serviceId is required");
  }

  const resp = await apiFetch(`/services/${serviceId}`);

  if (!resp.ok) {
    throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  }

  return resp.json();
}
