"use client"

import { ShieldCheck, Clock, XCircle, BarChart3 } from "lucide-react"
import { Stats } from "@/lib/api"

export function StatsCards({ stats }: { stats: Stats }) {
  const cards = [
    { label: "Total Scans", value: stats.total, icon: BarChart3 },
    { label: "Completed", value: stats.completed, icon: ShieldCheck },
    { label: "Running", value: stats.running, icon: Clock },
    { label: "Failed", value: stats.failed, icon: XCircle },
  ]

  return (
    <div className="grid grid-cols-4 gap-4">
      {cards.map((card, i) => (
        <div
          key={card.label}
          className="border p-4 flex items-center justify-between animate-slide-up"
          style={{ animationDelay: `${i * 60}ms`, animationFillMode: "backwards" }}
        >
          <div>
            <p className="text-xs text-muted-foreground">{card.label}</p>
            <p className="text-2xl font-semibold mt-0.5">{card.value}</p>
          </div>
          <card.icon className="h-5 w-5 text-muted-foreground" />
        </div>
      ))}
    </div>
  )
}
