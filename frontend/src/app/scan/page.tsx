"use client"

import { useEffect, useState, useCallback, Suspense } from "react"
import { useSearchParams, useRouter } from "next/navigation"
import Link from "next/link"
import { ArrowLeft, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { ModeToggle } from "@/components/mode-toggle"
import { ShareButton } from "@/components/share-button"
import { ExploitResults } from "@/components/exploit-results"
import { MalwareResults } from "@/components/malware-results"
import { api, Scan, ExploitResult, MalwareResult, parseResult } from "@/lib/api"
import { timeAgo } from "@/lib/utils"

function statusVariant(status: string) {
  switch (status) {
    case "completed": return "success" as const
    case "running": return "warning" as const
    case "failed": return "destructive" as const
    default: return "secondary" as const
  }
}

function ScanContent() {
  const searchParams = useSearchParams()
  const scanId = searchParams.get("id") || ""
  const [scan, setScan] = useState<Scan | null>(null)
  const [loading, setLoading] = useState(true)

  const fetchScan = useCallback(async () => {
    setLoading(true)
    try {
      const s = await api.getScan(scanId)
      setScan(s)
    } catch {} finally {
      setLoading(false)
    }
  }, [scanId])

  useEffect(() => {
    if (!scanId) {
      setLoading(false)
      return
    }
    fetchScan()

    const interval = setInterval(() => {
      if (!scanId) return
      api.getScan(scanId).then((s) => {
        setScan(s)
        if (s.status === "completed" || s.status === "failed") {
          clearInterval(interval)
        }
      }).catch(() => {})
    }, 3000)

    return () => clearInterval(interval)
  }, [scanId])

  if (!scanId) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <p className="text-sm text-muted-foreground">No scan specified</p>
      </div>
    )
  }

  if (loading && !scan) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (!scan) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <p className="text-sm text-muted-foreground">Scan not found</p>
      </div>
    )
  }

  const result = parseResult<ExploitResult | MalwareResult>(scan)

  return (
    <div className="min-h-screen">
      <header className="border-b px-6 h-12 flex items-center justify-between shrink-0">
        <div className="flex items-center gap-4">
          <Link href="/dashboard" className="flex items-center gap-2 hover:text-muted-foreground transition-colors">
            <ArrowLeft className="h-4 w-4" />
            <span className="text-xs">Back</span>
          </Link>
          <span className="text-xs text-muted-foreground truncate max-w-xs">
            {scan.source_detail || `Scan ${scan.id.slice(0, 8)}...`}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <ShareButton scan={scan} />
          <ModeToggle />
        </div>
      </header>

      <main className="px-6 py-6 max-w-4xl mx-auto space-y-6 animate-fade-in">
        <div className="flex items-center gap-3">
          <Badge variant={scan.type === "exploit" ? "default" : "outline"}>
            {scan.type === "exploit" ? "Exploit Scan" : "Malware Scan"}
          </Badge>
          <Badge variant={statusVariant(scan.status)}>{scan.status}</Badge>
          {scan.completed_at && (
            <span className="text-xs text-muted-foreground">{timeAgo(scan.completed_at)}</span>
          )}
        </div>

        {scan.status === "failed" && scan.error_message && (
          <div className="border border-destructive p-4">
            <p className="text-sm text-destructive">{scan.error_message}</p>
          </div>
        )}

        {scan.status === "running" && (
          <div className="border p-8 text-center">
            <Loader2 className="h-5 w-5 animate-spin mx-auto mb-2 text-muted-foreground" />
            <p className="text-sm text-muted-foreground">Scan in progress...</p>
          </div>
        )}

        {scan.status === "completed" && result && (
          <>
            {scan.type === "exploit" && "findings" in result && (
              <ExploitResults result={result as unknown as ExploitResult} />
            )}
            {scan.type === "malware" && "verdict" in result && (
              <MalwareResults result={result as unknown as MalwareResult} />
            )}
          </>
        )}
      </main>
    </div>
  )
}

export default function ScanPage() {
  return (
    <Suspense fallback={<div className="min-h-screen flex items-center justify-center"><Loader2 className="h-5 w-5 animate-spin text-muted-foreground" /></div>}>
      <ScanContent />
    </Suspense>
  )
}
