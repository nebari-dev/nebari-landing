// Temporary type definition for the data expected from the services API
// I am thinking of using something like valibot in order
// to validate the data being returned
export type Service = {
  id: string;
  name: string;
  status: string;
  description: string;
  category: string[];
  pinned: boolean;
  image: string;
};

/**
 * fetches all available services from the backend
 * @returns List of services to be displayed
 */
export
async function listServices(): Promise<Service[]> {
  
  // get the API_BASE_URL from the environment file
  const API_BASE_URL = import.meta.env.VITE_API_BASE_URL;

  // Guard against API url missing
  if (!API_BASE_URL) {
    throw new Error("VITE_API_BASE_URL is not set");
  }

  // Fetch the resource
  const resp = await fetch(`${API_BASE_URL}/services`, {
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
