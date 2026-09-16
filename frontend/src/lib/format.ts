import type { User } from "../api/types";

export function money(value: number) {
  return new Intl.NumberFormat("en", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(value);
}

export function shortDate(value?: string) {
  if (!value) {
    return "";
  }
  return new Intl.DateTimeFormat("en", {
    day: "2-digit",
    month: "short",
    hour: "2-digit",
    minute: "2-digit"
  }).format(new Date(value));
}

export function displayUser(user?: User) {
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
