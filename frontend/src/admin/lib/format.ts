import type { AdminUser } from "../api/types";

export function displayName(user: Pick<AdminUser, "firstName" | "lastName" | "username">) {
  const full = `${user.firstName} ${user.lastName}`.trim();
  return full || user.username;
}

const DATE_FMT = new Intl.DateTimeFormat(undefined, { dateStyle: "medium" });
const DATETIME_FMT = new Intl.DateTimeFormat(undefined, {
  dateStyle: "medium",
  timeStyle: "short",
});

export function formatDate(iso: string | null | undefined): string {
  if (!iso) return "Never";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "Unknown" : DATE_FMT.format(d);
}

export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return "Never";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "Unknown" : DATETIME_FMT.format(d);
}

export function pluralize(count: number, singular: string, plural = `${singular}s`) {
  return `${count} ${count === 1 ? singular : plural}`;
}
