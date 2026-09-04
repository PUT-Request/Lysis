"use client"

import { ThemeProvider } from "@/components/theme-provider"

export function RootLayoutClient({ children }: { children: React.ReactNode }) {
  return <ThemeProvider>{children}</ThemeProvider>
}
