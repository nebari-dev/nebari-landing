// Seed data for the MSW in-memory store. Edit freely — this is the only
// place that defines what the SPA renders under VITE_USE_MOCKS=1.

import type { Service } from "../api/listServices";

export type AccessRequest = {
  id: string;
  serviceUID: string;
  serviceName: string;
  targetOwner: string;
  userID: string;
  userEmail: string;
  message: string;
  status: "pending" | "approved" | "denied" | "revoked";
  requestedAt: string;
  resolvedAt: string;
  resolvedBy: string;
  resolvedByName: string;
  resolvedByEmail: string;
  expiresAt: string;
};

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
    category: ["ML"],
    pinned: false,
    image: "",
    url: "https://mlflow.example.com",
  },
];

export const seedAccessRequests: AccessRequest[] = [
  {
    id: "req-1",
    serviceUID: "svc-mlflow",
    serviceName: "MLflow",
    targetOwner: "nebari-system/mlflow",
    userID: "alice",
    userEmail: "alice@example.com",
    message: "Need MLflow for the recsys experiments.",
    status: "pending",
    requestedAt: new Date(Date.now() - 1000 * 60 * 30).toISOString(),
    resolvedAt: "",
    resolvedBy: "",
    resolvedByName: "",
    resolvedByEmail: "",
    expiresAt: "",
  },
];

export const seedCategories: Record<string, string> = {
  Notebooks: "Interactive computing",
  IDE: "Code editing",
  Monitoring: "Observability",
  ML: "Machine learning",
};
