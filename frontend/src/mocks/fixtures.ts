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
    description:
      "Notebooks and terminals for data science and machine learning work, each running in its own server on the cluster.\n\nPick a server size when you start, install packages into your own environments, and share notebooks with your team. Your server shuts down after a period of inactivity, and your files are kept for the next session.",
    category: ["Notebooks"],
    pinned: true,
    image: "",
    url: "https://jupyter.example.com",
    version: "5.2.1",
    docsUrl: "https://jupyterhub.readthedocs.io/en/stable/",
    sessionCount: 2,
  },
  {
    id: "svc-vscode",
    name: "VS Code Server",
    status: "Healthy",
    description:
      "Browser-based VS Code with shared workspaces, extensions, and an integrated terminal.\n\nOpen the same files and environments you use in JupyterHub, debug Python with breakpoints, and use Git from the editor without setting anything up locally.",
    category: ["IDE"],
    pinned: false,
    image: "",
    url: "https://code.example.com",
    version: "4.96.4",
    docsUrl: "https://coder.com/docs/code-server",
    sessionCount: 1,
  },
  {
    id: "svc-grafana",
    name: "Grafana",
    status: "Unhealthy",
    health: "degraded",
    description:
      "Dashboards for cluster metrics, logs and traces.\n\nSee how much CPU, memory and GPU your workloads use, follow logs from a running job, and set up alerts for the things you care about. Dashboards for the platform itself are installed by default.",
    category: ["Monitoring"],
    pinned: false,
    image: "",
    url: "https://grafana.example.com",
    version: "11.4.0",
    docsUrl: "https://grafana.com/docs/grafana/latest/",
  },
  {
    id: "svc-mlflow",
    name: "MLflow",
    status: "Unknown",
    description:
      "Track experiments, compare runs and register models.\n\nLog parameters, metrics and artifacts from any notebook or job, compare runs side by side, and promote the best model to the registry so others can deploy it.",
    category: ["ML", "Monitoring"],
    pinned: true,
    image: "",
    url: "https://mlflow.example.com",
    version: "2.19.0",
    docsUrl: "https://mlflow.org/docs/latest/",
  },
  {
    id: "svc-nebi",
    name: "Nebi",
    status: "Healthy",
    description:
      "Create, share and reuse reproducible Python environments.\n\nBuild conda and pixi environments from a spec, pin versions so everyone gets the same packages, and roll back to an earlier version of an environment when an upgrade breaks something.",
    category: ["Environments"],
    pinned: false,
    image: "",
    url: "https://nebi.example.com",
    version: "0.4.2",
    docsUrl: "https://nebi.nebari.dev/docs",
  },
  {
    id: "svc-argocd",
    name: "Argo CD",
    status: "Healthy",
    description:
      "Argo CD keeps the apps running on this platform in sync with what’s in Git. Teams describe how an app should be deployed in a Git repository, and Argo CD makes sure the cluster matches it, applying changes automatically when someone commits.\n\nUse it to see what’s deployed and whether each app is healthy, check what changed and when, and roll back to an earlier version if a deployment goes wrong. Because every change goes through Git, there’s always a record of who changed what.",
    category: ["Platform"],
    pinned: false,
    image: "",
    url: "https://argocd.example.com",
    version: "v2.13.0",
    docsUrl: "https://argo-cd.readthedocs.io/en/stable/",
  },
  {
    id: "svc-dask",
    name: "Dask Gateway",
    status: "Healthy",
    description:
      "Spin up Dask clusters for large, parallel computations.\n\nStart a cluster from a notebook with a few lines of Python, scale workers up and down as the job needs, and watch progress live in the Dask dashboard. Clusters shut down when you're done so they don't hold on to resources.",
    category: ["Notebooks"],
    pinned: false,
    image: "",
    url: "https://dask.example.com",
    version: "2024.1.0",
    docsUrl: "https://gateway.dask.org/",
    sessionCount: 3,
  },
  {
    id: "svc-keycloak",
    name: "Keycloak",
    status: "Healthy",
    description:
      "Single sign-on, users and groups for the whole platform.\n\nAdmins manage who can sign in and which groups they belong to; group membership decides which services each person sees on this page.",
    category: ["Platform"],
    pinned: false,
    image: "",
    url: "https://keycloak.example.com",
    version: "26.0.7",
    docsUrl: "https://www.keycloak.org/documentation",
  },
  {
    id: "svc-prometheus",
    name: "Prometheus",
    status: "Unhealthy",
    health: "down",
    description:
      "Collects cluster metrics and sends alerts when something breaks.\n\nGrafana's dashboards read from Prometheus, so they show gaps while it's down. Alert rules cover node health, storage, and failing workloads.",
    category: ["Monitoring"],
    pinned: false,
    image: "",
    url: "https://prometheus.example.com",
    version: "3.1.0",
    docsUrl: "https://prometheus.io/docs/",
    sessionCount: 1,
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
