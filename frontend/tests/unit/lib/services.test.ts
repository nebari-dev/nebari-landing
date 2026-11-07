import { describe, expect, it } from "vitest";
import type { Service } from "@/api/listServices";
import { filterServices, getCategories, sortServices, usesRicherLayout } from "@/lib/services";

function service(overrides: Partial<Service> & Pick<Service, "id" | "name">): Service {
  return {
    status: "Healthy",
    description: "",
    category: [],
    pinned: false,
    url: `https://example.com/${overrides.id}`,
    ...overrides,
  };
}

const jupyter = service({
  id: "jupyter",
  name: "JupyterHub",
  description: "Notebook platform",
  category: ["Data science"],
});
const grafana = service({
  id: "grafana",
  name: "grafana",
  category: ["Monitoring"],
  status: "Unhealthy",
});
const argo = service({ id: "argo", name: "Argo CD", category: ["Platform"], status: "Unknown" });
const dask = service({ id: "dask", name: "Dask Gateway", category: ["Data science"] });

const names = (services: Service[]) => services.map((item) => item.name);

describe("sortServices", () => {
  it("sorts by name in either direction, ignoring case", () => {
    const services = [jupyter, grafana, argo, dask];

    expect(names(sortServices(services, { key: "name", direction: "asc" }))).toEqual([
      "Argo CD",
      "Dask Gateway",
      "grafana",
      "JupyterHub",
    ]);
    expect(names(sortServices(services, { key: "name", direction: "desc" }))).toEqual([
      "JupyterHub",
      "grafana",
      "Dask Gateway",
      "Argo CD",
    ]);
  });

  it("sorts by category and breaks ties by name", () => {
    expect(
      names(sortServices([jupyter, grafana, argo, dask], { key: "category", direction: "asc" })),
    ).toEqual(["Dask Gateway", "JupyterHub", "grafana", "Argo CD"]);
  });

  it("sorts available services ahead of unknown and unavailable ones", () => {
    expect(
      names(sortServices([grafana, argo, jupyter], { key: "status", direction: "asc" })),
    ).toEqual(["JupyterHub", "Argo CD", "grafana"]);
    expect(
      names(sortServices([grafana, argo, jupyter], { key: "status", direction: "desc" })),
    ).toEqual(["grafana", "Argo CD", "JupyterHub"]);
  });

  it("does not mutate its input", () => {
    const services = [jupyter, argo];
    sortServices(services, { key: "name", direction: "asc" });

    expect(names(services)).toEqual(["JupyterHub", "Argo CD"]);
  });
});

describe("filterServices", () => {
  const services = [jupyter, grafana, argo, dask];

  it("matches the query against name, description, and category", () => {
    expect(names(filterServices(services, "notebook", []))).toEqual(["JupyterHub"]);
    expect(names(filterServices(services, "  MONITORING ", []))).toEqual(["grafana"]);
  });

  it("keeps services in any of the selected categories", () => {
    expect(names(filterServices(services, "", ["Data science", "Monitoring"]))).toEqual([
      "JupyterHub",
      "grafana",
      "Dask Gateway",
    ]);
  });

  it("combines the query with the category selection", () => {
    expect(names(filterServices(services, "dask", ["Data science"]))).toEqual(["Dask Gateway"]);
  });
});

describe("usesRicherLayout", () => {
  const many = (count: number) =>
    Array.from({ length: count }, (_, index) => service({ id: `${index}`, name: `S${index}` }));

  it("switches to the richer layout at 6 services", () => {
    expect(usesRicherLayout(many(0))).toBe(false);
    expect(usesRicherLayout(many(5))).toBe(false);
    expect(usesRicherLayout(many(6))).toBe(true);
  });
});

describe("getCategories", () => {
  it("lists each category once, A–Z", () => {
    expect(getCategories([jupyter, grafana, argo, dask])).toEqual([
      "Data science",
      "Monitoring",
      "Platform",
    ]);
  });
});
