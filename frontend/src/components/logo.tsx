"use client"

import { useTheme } from "@/components/theme-provider"

export function Logo({ className }: { className?: string }) {
  const { theme } = useTheme()
  const src = theme === "dark" ? "/dark-mode-logo.png" : "/light-mode-logo.png"
  return (
    <span className={`inline-flex items-center gap-2 ${className || ""}`}>
      <img src={src} alt="Lysis" className="h-5 w-auto" />
      <span className="font-semibold text-sm">Lysis</span>
    </span>
  )
}
