import { apiFetch } from "./client";

export type Pin = {
  id: string;
};

export async function getPinById(serviceId: string): Promise<Pin> {
  if (!serviceId) {
    throw new Error("serviceId is required");
  }

  const resp = await apiFetch(`/pins/${serviceId}`);

  if (!resp.ok) {
    throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  }

  return resp.json();
}

export async function putPin(serviceId: string): Promise<void> {
  if (!serviceId) {
    throw new Error("serviceId is required");
  }

  const resp = await apiFetch(`/pins/${serviceId}`, {
    method: "PUT",
  });

  if (!resp.ok) {
    throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  }
}

export async function deletePin(serviceId: string): Promise<void> {
  if (!serviceId) {
    throw new Error("serviceId is required");
  }

  const resp = await apiFetch(`/pins/${serviceId}`, {
    method: "DELETE",
  });

  if (!resp.ok) {
    throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  }
}
