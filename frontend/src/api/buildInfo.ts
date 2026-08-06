export type BuildInfo = {
  version: string;
  commit: string;
  lastUpdated: string | null;
};

export async function getBuildInfo(signal?: AbortSignal): Promise<BuildInfo> {
  const response = await fetch("/build-info.json", { signal, cache: "no-store" });

  if (!response.ok) {
    throw new Error(`Failed to load build information: ${response.status}`);
  }

  return response.json() as Promise<BuildInfo>;
}
