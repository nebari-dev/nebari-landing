import type { Service } from "../api/listServices";
import type { Notification } from "../api/notifications";

export const mockUser = {
  name: "Test User",
  email: "test.user@example.com",
};

export const mockServices: Service[] = [
  {
    id: "svc-1",
    name: "JupyterHub",
    status: "Healthy",
    description: "Notebook platform",
    category: ["Data Science"],
    pinned: true,
    image: "",
    url: "https://example.com/jupyterhub",
  },
  {
    id: "svc-2",
    name: "Grafana",
    status: "Unhealthy",
    description: "Metrics dashboards",
    category: ["Monitoring"],
    pinned: false,
    image: "",
    url: "https://example.com/grafana",
  },
  {
    id: "svc-3",
    name: "Admin Panel",
    status: "Unknown",
    description: "Administrative tools",
    category: ["Platform"],
    pinned: false,
    image: "",
    url: "https://example.com/admin",
  },
];

export const mockNotifications: Notification[] = [
  {
    id: "notif-1",
    image: "",
    title: "JupyterHub is back online!",
    message: "JupyterHub is back online and ready to use.",
    read: false,
    createdAt: new Date().toISOString(),
  },
  {
    id: "notif-2",
    image: "",
    title: "Scheduled maintenance planned.",
    message: "Maintenance will occur on the first Saturday of each month.",
    read: true,
    createdAt: new Date().toISOString(),
  },
];
