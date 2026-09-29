import { Link, Navigate, Route, Routes } from "react-router";
import { signIn } from "@/auth/keycloak";
import type { User } from "@/auth/user";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useCallerIdentity } from "@/hooks/useCallerIdentity";
import { AdminLayout } from "./components/AdminLayout";
import { EmptyState } from "./components/EmptyState";
import { ActivityPage } from "./pages/ActivityPage";
import { GroupDetailPage } from "./pages/GroupDetailPage";
import { GroupsPage } from "./pages/GroupsPage";
import { HealthPage } from "./pages/HealthPage";
import { OverviewPage } from "./pages/OverviewPage";
import { PackDetailPage } from "./pages/PackDetailPage";
import { PacksPage } from "./pages/PacksPage";
import { RoleDetailPage } from "./pages/RoleDetailPage";
import { RolesPage } from "./pages/RolesPage";
import { ServiceDetailPage } from "./pages/ServiceDetailPage";
import { ServicesPage } from "./pages/ServicesPage";
import { UserDetailPage } from "./pages/UserDetailPage";
import { UsersPage } from "./pages/UsersPage";

type AdminAppProps = {
  user: User | null;
};

/**
 * Entry point for `/admin/*`. Gates on the server-trusted identity: anonymous
 * visitors are asked to sign in, non-admins see a polite dead end, admins get
 * the section routes. The routes never 404 for non-admins so deep links stay
 * shareable between admins.
 */
export default function AdminApp({ user }: AdminAppProps) {
  const { isAdmin, isLoading, error } = useCallerIdentity(user);

  if (!user) {
    return (
      <AdminGate>
        <EmptyState
          title="Sign in to continue"
          description="The administration area is available to platform administrators."
          action={
            <Button type="button" onClick={() => signIn()}>
              Sign in
            </Button>
          }
        />
      </AdminGate>
    );
  }

  if (isLoading) {
    return (
      <AdminGate>
        <output aria-label="Checking permissions" className="flex flex-col gap-3">
          <Skeleton className="h-6 w-48" />
          <Skeleton className="h-4 w-80" />
          <Skeleton className="h-40" shape="block" />
        </output>
      </AdminGate>
    );
  }

  if (error) {
    return (
      <AdminGate>
        <EmptyState
          variant="error"
          title="Could not verify your permissions"
          description={error.message}
        />
      </AdminGate>
    );
  }

  if (!isAdmin) {
    return (
      <AdminGate>
        <EmptyState
          title="You don't have access to administration"
          description="Only members of the admin group can manage users, groups and roles. Ask a platform administrator if you believe you should have access."
          action={
            <Button variant="outline" render={<Link to="/" />}>
              Back to the Launchpad
            </Button>
          }
        />
      </AdminGate>
    );
  }

  return (
    <Routes>
      <Route element={<AdminLayout />}>
        <Route index element={<Navigate to="overview" replace />} />
        <Route path="overview" element={<OverviewPage />} />
        <Route path="packs" element={<PacksPage />} />
        <Route path="packs/:name" element={<PackDetailPage />} />
        <Route path="activity" element={<ActivityPage />} />
        <Route path="health" element={<HealthPage />} />
        <Route path="users" element={<UsersPage />} />
        <Route path="users/:id" element={<UserDetailPage />} />
        <Route path="groups" element={<GroupsPage />} />
        <Route path="groups/:id" element={<GroupDetailPage />} />
        <Route path="roles" element={<RolesPage />} />
        <Route path="roles/:name" element={<RoleDetailPage />} />
        <Route path="services" element={<ServicesPage />} />
        <Route path="services/:id" element={<ServiceDetailPage />} />
        <Route path="*" element={<Navigate to="overview" replace />} />
      </Route>
    </Routes>
  );
}

function AdminGate({ children }: { children: React.ReactNode }) {
  return (
    <section aria-labelledby="admin-gate-title" className="mx-auto max-w-3xl px-4 py-16 sm:px-12">
      <h1 id="admin-gate-title" className="sr-only">
        Administration
      </h1>
      {children}
    </section>
  );
}
