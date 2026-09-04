import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatDate(dateStr: string): string {
  const d = parseDate(dateStr)
  return d.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

export function timeAgo(dateStr: string): string {
  const d = parseDate(dateStr)
  const now = new Date()
  const diff = Math.floor((now.getTime() - d.getTime()) / 1000)

  if (diff < 60) return "just now"
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

function parseDate(dateStr: string): Date {
  if (dateStr.includes("T")) {
    if (dateStr.endsWith("Z") || dateStr.includes("+") || dateStr.includes("-") && dateStr.lastIndexOf("-") > 10) {
      return new Date(dateStr)
    }
    return new Date(dateStr + "Z")
  }
  return new Date(dateStr.replace(" ", "T") + "Z")
}

export function getBaseUrl(): string {
  if (typeof window !== "undefined") {
    return window.location.origin
  }
  return ""
}
