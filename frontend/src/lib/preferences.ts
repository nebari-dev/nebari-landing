/** Per-browser view preferences; reads return `undefined` when storage is unavailable. */

export type ServicesLayout = "simple" | "richer";
export type ServicesView = "grid" | "table";

const LAYOUT_KEY = "launchpad:servicesLayout";
const VIEW_KEY = "launchpad:servicesView";

function read<T extends string>(key: string, allowed: readonly T[]): T | undefined {
  try {
    const value = window.localStorage.getItem(key);
    return allowed.find((item) => item === value);
  } catch {
    return undefined;
  }
}

function write(key: string, value: string) {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    // Storage unavailable.
  }
}

/** The layout the services last rendered in. */
export const readServicesLayout = () => read<ServicesLayout>(LAYOUT_KEY, ["simple", "richer"]);
export const writeServicesLayout = (layout: ServicesLayout) => write(LAYOUT_KEY, layout);

/** The Grid / Table choice in the richer layout. */
export const readServicesView = () => read<ServicesView>(VIEW_KEY, ["grid", "table"]);
export const writeServicesView = (view: ServicesView) => write(VIEW_KEY, view);
