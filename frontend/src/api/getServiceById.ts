import { apiFetch } from "./client";

export type Service = {
  id: string;
  name: string;
  status: string;
  description: string;
  category: string[];
  pinned: boolean;
  image: string;
};

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
