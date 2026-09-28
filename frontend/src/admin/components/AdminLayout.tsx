import { KeyRound, LayoutGrid, Users, UsersRound } from "lucide-react";
import type { ReactNode } from "react";
import { NavLink, Outlet } from "react-router";
import { cn } from "@/lib/utils";

const SECTIONS: { to: string; label: string; icon: ReactNode }[] = [
  { to: "/admin/users", label: "Users", icon: <Users aria-hidden="true" /> },
  { to: "/admin/groups", label: "Groups", icon: <UsersRound aria-hidden="true" /> },
  { to: "/admin/roles", label: "Roles", icon: <KeyRound aria-hidden="true" /> },
  { to: "/admin/services", label: "Services", icon: <LayoutGrid aria-hidden="true" /> },
];

/**
 * Shell for every admin page: a title row and the section switcher, then the
 * routed page. The switcher is real navigation (links with `aria-current`),
 * not tabs, because each section is its own URL.
 */
export function AdminLayout() {
  return (
    <div className="px-4 py-6 sm:px-12">
      <div className="flex flex-col gap-4 border-b border-border pb-4 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">Administration</h1>
          <p className="text-sm text-muted-foreground">
            Manage who can reach which services. Identity lives in Keycloak; service gates live in
            each NebariApp.
          </p>
        </div>
        <nav aria-label="Admin sections">
          <ul className="flex flex-wrap gap-1 rounded-md bg-muted p-1">
            {SECTIONS.map((s) => (
              <li key={s.to}>
                <NavLink
                  to={s.to}
                  className={({ isActive }) =>
                    cn(
                      "inline-flex h-8 items-center gap-1.5 rounded-sm border border-transparent px-2.5 text-sm font-medium outline-none focus-visible:ring-2 focus-visible:ring-ring [&_svg]:size-4",
                      isActive
                        ? "border-border-strong bg-card text-foreground shadow-[0_1px_3px_0_rgba(0,0,0,0.10)]"
                        : "text-muted-foreground-strong hover:text-foreground",
                    )
                  }
                >
                  {s.icon}
                  {s.label}
                </NavLink>
              </li>
            ))}
          </ul>
        </nav>
      </div>
      <div className="pt-6">
        <Outlet />
      </div>
    </div>
  );
}
