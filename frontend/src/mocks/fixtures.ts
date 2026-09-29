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
    userID: "alice",
    userEmail: "alice@example.com",
    message: "Need MLflow for the recsys experiments.",
    status: "pending",
    requestedAt: new Date(Date.now() - 1000 * 60 * 30).toISOString(),
    resolvedAt: "",
    resolvedBy: "",
  },
  {
    id: "req-2",
    serviceUID: "svc-superset",
    serviceName: "Superset",
    userID: "ivan.ito",
    userEmail: "ivan.ito@example.com",
    message: "Quarterly finance dashboards.",
    status: "pending",
    requestedAt: new Date(Date.now() - 1000 * 60 * 60 * 5).toISOString(),
    resolvedAt: "",
    resolvedBy: "",
  },
  {
    id: "req-3",
    serviceUID: "svc-grafana",
    serviceName: "Grafana",
    userID: "quinn.eriksen",
    userEmail: "quinn.eriksen@example.com",
    message: "",
    status: "approved",
    requestedAt: new Date(Date.now() - 1000 * 60 * 60 * 30).toISOString(),
    resolvedAt: new Date(Date.now() - 1000 * 60 * 60 * 26).toISOString(),
    resolvedBy: "alice",
  },
  {
    id: "req-4",
    serviceUID: "svc-keycloak",
    serviceName: "Keycloak",
    userID: "gabriel.gomez",
    userEmail: "gabriel.gomez@example.com",
    message: "Need to reset a teammate's password.",
    status: "denied",
    requestedAt: new Date(Date.now() - 1000 * 60 * 60 * 50).toISOString(),
    resolvedAt: new Date(Date.now() - 1000 * 60 * 60 * 48).toISOString(),
    resolvedBy: "alice",
  },
];

export const seedCategories: Record<string, string> = {
  Notebooks: "Interactive computing",
  IDE: "Code editing",
  Monitoring: "Observability",
  ML: "Machine learning",
};
