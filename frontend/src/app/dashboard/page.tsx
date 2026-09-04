"use client"

import { useEffect, useState, useCallback, useRef } from "react"
import { useRouter } from "next/navigation"
import Link from "next/link"
import { Plus, LogOut } from "lucide-react"
import { StatsCards } from "@/components/stats-cards"
import { ScansTable } from "@/components/scans-table"
import { NewScanDialog } from "@/components/new-scan-dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { ModeToggle } from "@/components/mode-toggle"
import { Logo } from "@/components/logo"
import { api, Stats, PaginatedScans, clearToken, isAuthenticated } from "@/lib/api"

export default function DashboardPage() {
  const router = useRouter()
  const [stats, setStats] = useState<Stats | null>(null)
  const [scans, setScans] = useState<PaginatedScans | null>(null)
  const [showNewScan, setShowNewScan] = useState(false)
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState("")
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const fetchData = useCallback(async (page = 1, q = search) => {
    try {
      const [s, sc] = await Promise.all([api.stats(), api.listScans(page, q)])
      setStats(s)
      setScans(sc)
    } catch {
      if (!isAuthenticated()) {
        clearToken()
        router.push("/login")
      }
    } finally {
      setLoading(false)
    }
  }, [search])

  useEffect(() => {
    if (!isAuthenticated()) {
      router.push("/login")
      return
    }
    fetchData()
    const interval = setInterval(() => fetchData(), 5000)
    return () => clearInterval(interval)
  }, [])

  function handleSearch(val: string) {
    setSearch(val)
    if (debounceRef.current !== null) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => fetchData(1, val), 300)
  }

  function handleLogout() {
    clearToken()
    router.push("/")
  }

  return (
    <div className="min-h-screen">
      <header className="border-b px-6 h-12 flex items-center justify-between shrink-0">
        <div className="flex items-center gap-4">
          <Link href="/dashboard" className="flex items-center gap-3">
            <Logo className="h-5 w-auto" />
          </Link>
          <span className="text-xs text-muted-foreground">Dashboard</span>
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" onClick={() => setShowNewScan(true)}>
            <Plus className="h-3.5 w-3.5" />
            New Scan
          </Button>
          <ModeToggle />
          <Button variant="ghost" size="sm" onClick={handleLogout}>
            <LogOut className="h-3.5 w-3.5" />
          </Button>
        </div>
      </header>

      <main className="px-6 py-6 max-w-5xl mx-auto space-y-6 animate-fade-in">
        {stats && <StatsCards stats={stats} />}
        {scans && (
          <ScansTable
            scans={scans}
            onRefresh={(p) => fetchData(p)}
            search={search}
            onSearch={handleSearch}
          />
        )}
        {loading && (
          <div className="space-y-6 animate-fade-in">
            <div className="grid grid-cols-4 gap-4">
              {[1, 2, 3, 4].map((i) => (
                <div key={i} className="border p-4 space-y-3">
                  <div className="skeleton h-3 w-16" />
                  <div className="skeleton h-7 w-10" />
                </div>
              ))}
            </div>
            <div className="border">
              <div className="px-5 py-3 border-b">
                <div className="skeleton h-3 w-12" />
              </div>
              <div className="p-5 space-y-2">
                {[1, 2, 3].map((i) => (
                  <div key={i} className="skeleton h-8 w-full" style={{ opacity: 1 - i * 0.2 }} />
                ))}
              </div>
            </div>
          </div>
        )}
      </main>

      <NewScanDialog open={showNewScan} onClose={() => setShowNewScan(false)} onCreated={() => fetchData()} />
    </div>
  )
}
