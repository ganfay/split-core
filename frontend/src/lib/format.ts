import type { User } from "../api/types";

export function money(value: number) {
  const safe = Number.isFinite(value) ? value : 0;
  return new Intl.NumberFormat("en", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(safe);
}

export function shortDate(value?: string | null) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return new Intl.DateTimeFormat("en", {
    day: "2-digit",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function fullDateTime(value?: string | null) {
  if (!value) {
    return "Unknown date";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "Unknown date";
  }
  return new Intl.DateTimeFormat("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function displayUser(user?: User | null) {
  if (!user) {
    return "Unknown";
  }
  if (user.username) {
    return `@${user.username}`;
  }
  if (user.first_name && user.first_name !== ".") {
    return user.first_name;
  }
  return `User_${String(user.id).slice(-4)}`;
}
