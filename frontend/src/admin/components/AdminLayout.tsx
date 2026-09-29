import { Gauge, Inbox, KeyRound, LayoutGrid, Users, UsersRound } from "lucide-react";
import type { ReactNode } from "react";
import { Link, Outlet, useLocation } from "react-router";
import { NavLink } from "@/components/ui/navigation-menu";

const SECTIONS: { to: string; label: string; icon: ReactNode }[] = [
  { to: "/admin/overview", label: "Overview", icon: <Gauge aria-hidden="true" /> },
  { to: "/admin/users", label: "Users", icon: <Users aria-hidden="true" /> },
  { to: "/admin/groups", label: "Groups", icon: <UsersRound aria-hidden="true" /> },
  { to: "/admin/roles", label: "Roles", icon: <KeyRound aria-hidden="true" /> },
  { to: "/admin/services", label: "Services", icon: <LayoutGrid aria-hidden="true" /> },
  { to: "/admin/requests", label: "Requests", icon: <Inbox aria-hidden="true" /> },
];

/**
 * Shell for every admin page: a title row and the section switcher, then the
 * routed page. The switcher is real navigation (registry `NavLink`s rendered
 * as router links, with `aria-current` on the active section), not tabs,
 * because each section is its own URL.
 */
export function AdminLayout() {
  const { pathname } = useLocation();
  return (
    <div className="px-4 py-6 sm:px-12">
      <div className="flex flex-col gap-4 border-b border-border pb-4 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">Administration</h1>
          <p className="text-sm text-muted-foreground">
            Identity lives in Keycloak; service gates live in each NebariApp.
          </p>
        </div>
        <nav aria-label="Admin sections" className="flex flex-wrap items-center gap-1">
          {SECTIONS.map((s) => (
            <NavLink
              key={s.to}
              active={pathname === s.to || pathname.startsWith(`${s.to}/`)}
              icon={s.icon}
              render={<Link to={s.to} />}
            >
              {s.label}
            </NavLink>
          ))}
        </nav>
      </div>
      <div className="pt-6 motion-safe:animate-fade-in">
        <Outlet />
      </div>
    </div>
  );
}
