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

  // get the API_BASE_URL from the environment file
  const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? "").replace(/\/$/, "");

  // Guard against API url missing
  if (!API_BASE_URL) {
    throw new Error("VITE_API_BASE_URL is not set");
  }

  // Guard against serviceId missing
  if (!serviceId) {
    throw new Error("serviceId is required");
  }

  // Fetch the resource
  const resp = await fetch(`${API_BASE_URL}/services/${serviceId}`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
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
