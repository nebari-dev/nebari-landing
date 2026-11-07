// Seed data for the MSW in-memory store. Edit freely — this is the only
// place that defines what the SPA renders under VITE_USE_MOCKS=1.

import type { Service } from "../api/listServices";

export type AccessRequest = {
  id: string;
  serviceUID: string;
  serviceName: string;
  userID: string;
  userEmail: string;
  message: string;
  status: "pending" | "approved" | "denied";
  requestedAt: string;
  resolvedAt: string;
  resolvedBy: string;
};

// Six or more services render the richer layout; fewer than six render the simple layout.
export const seedServices: Service[] = [
  {
    id: "svc-jupyter",
    name: "JupyterHub",
    status: "Healthy",
    description: "Multi-user notebook server backed by Kubernetes.",
    category: ["Notebooks"],
    pinned: true,
    image: "",
    url: "https://jupyter.example.com",
  },
  {
    id: "svc-vscode",
    name: "VS Code Server",
    status: "Healthy",
    description: "Browser-based VS Code with shared workspaces.",
    category: ["IDE"],
    pinned: false,
    image: "",
    url: "https://code.example.com",
  },
  {
    id: "svc-grafana",
    name: "Grafana",
    status: "Healthy",
    description: "Dashboards for cluster and workload metrics.",
    category: ["Monitoring"],
    pinned: false,
    image: "",
    url: "https://grafana.example.com",
  },
  {
    id: "svc-mlflow",
    name: "MLflow",
    status: "Unknown",
    description: "Track experiments, package and deploy models.",
    category: ["ML", "Monitoring"],
    pinned: true,
    image: "",
    url: "https://mlflow.example.com",
  },
  {
    id: "svc-nebi",
    name: "Nebi",
    status: "Healthy",
    description: "Create, share and reuse reproducible Python environments.",
    category: ["Environments"],
    pinned: false,
    image: "",
    url: "https://nebi.example.com",
  },
  {
    id: "svc-argocd",
    name: "Argo CD",
    status: "Healthy",
    description: "Keeps the apps on this platform in sync with Git.",
    category: ["Platform"],
    pinned: false,
    image: "",
    url: "https://argocd.example.com",
  },
  {
    id: "svc-dask",
    name: "Dask Gateway",
    status: "Healthy",
    description: "Spin up Dask clusters for large, parallel computations.",
    category: ["Notebooks"],
    pinned: false,
    image: "",
    url: "https://dask.example.com",
  },
  {
    id: "svc-keycloak",
    name: "Keycloak",
    status: "Healthy",
    description: "Single sign-on, users and groups for the whole platform.",
    category: ["Platform"],
    pinned: false,
    image: "",
    url: "https://keycloak.example.com",
  },
  {
    id: "svc-prometheus",
    name: "Prometheus",
    status: "Unhealthy",
    description: "Collects cluster metrics and sends alerts when something breaks.",
    category: ["Monitoring"],
    pinned: false,
    image: "",
    url: "https://prometheus.example.com",
  },
];

export const seedAccessRequests: AccessRequest[] = [
  {
    id: "req-1",
    serviceUID: "svc-mlflow",
    serviceName: "MLflow",
    userID: "alice",
    userEmail: "alice@example.com",
    message: "Need MLflow for the recsys experiments.",
    status: "pending",
    requestedAt: new Date(Date.now() - 1000 * 60 * 30).toISOString(),
    resolvedAt: "",
    resolvedBy: "",
  },
];

export const seedCategories: Record<string, string> = {
  Notebooks: "Interactive computing",
  IDE: "Code editing",
  Monitoring: "Observability",
  ML: "Machine learning",
  Environments: "Software environments",
  Platform: "Platform operations",
};
