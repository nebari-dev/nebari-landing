import type { ReactNode } from "react";

import AppAccordion from "../accordion";

import "./index.scss";

export default function Content(): ReactNode {
  const services = [
    {
      id: "svc-1",
      image: "",
      name: "Payments",
      status: "Healthy",
      description: "Process transactions and manage payment flows.",
      category: ["Finance", "Core"],
      pinned: true,
    },
    {
      id: "svc-2",
      image: "",
      name: "Notifications",
      status: "Active",
      description: "Send email, SMS, and in-app notifications.",
      category: ["Messaging", "Platform"],
      pinned: false,
    },
    {
      id: "svc-3",
      image: "",
      name: "User Directory",
      status: "Healthy",
      description: "Manage users, groups, and directory sync.",
      category: ["Identity", "Admin"],
      pinned: true,
    },
    {
      id: "svc-4",
      image: "",
      name: "Analytics",
      status: "Degraded",
      description: "Dashboards and usage metrics for services.",
      category: ["Reporting", "Insights"],
      pinned: false,
    },
    {
      id: "svc-5",
      image: "",
      name: "Audit Logs",
      status: "Healthy",
      description: "Track system events and user activity history.",
      category: ["Security", "Compliance"],
      pinned: false,
    },
    {
      id: "svc-6",
      image: "",
      name: "Developer Portal",
      status: "Active",
      description: "Docs, API keys, and developer tooling.",
      category: ["Developer", "Platform"],
      pinned: false,
    },
  ];

  const pinnedServices = services.filter((service) => service.pinned);

  return (
    <main id="main-content" className="app-content">
      <h1 className="launchpad-title">Launchpad</h1>

      <AppAccordion
        pinnedServices={pinnedServices}
        services={services}
      />
    </main>
  );
}
