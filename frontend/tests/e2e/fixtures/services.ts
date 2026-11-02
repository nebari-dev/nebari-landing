export type MockService = {
  id: string;
  name: string;
  status: string;
  description: string;
  category: string[];
  pinned: boolean;
  image: string;
  url: string;
};

function mockService(
  overrides: Partial<MockService> & Pick<MockService, "id" | "name">,
): MockService {
  return {
    status: "Healthy",
    description: "",
    category: [],
    pinned: false,
    image: "",
    url: `https://example.com/${overrides.id}`,
    ...overrides,
  };
}

const jupyterHub = mockService({
  id: "svc-1",
  name: "JupyterHub",
  description: "Notebook platform",
  category: ["Data Science"],
  pinned: true,
  url: "https://example.com/jupyterhub",
});

/** Six services, the minimum for the richer layout, listed out of name order. */
export const richerServices: MockService[] = [
  jupyterHub,
  mockService({
    id: "svc-nebi",
    name: "Nebi",
    description: "Reproducible Python environments",
    category: ["Environments"],
  }),
  mockService({
    id: "svc-mlflow",
    name: "MLflow",
    description: "Experiment tracking and model registry",
    category: ["Machine Learning"],
  }),
  mockService({
    id: "svc-grafana",
    name: "Grafana",
    description: "Dashboards for cluster metrics",
    category: ["Monitoring"],
  }),
  mockService({
    id: "svc-argo",
    name: "Argo CD",
    description: "Deploys apps from Git",
    category: ["Platform"],
  }),
  mockService({
    id: "svc-dask",
    name: "Dask Gateway",
    description: "Scalable Python compute",
    category: ["Data Science"],
    status: "Unhealthy",
  }),
];

/** Five services: one short of the richer layout. */
export const simpleServices: MockService[] = richerServices.slice(0, 5);
