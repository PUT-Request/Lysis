"use client"

import { useEffect, useState, Suspense } from "react"
import { useSearchParams } from "next/navigation"
import Link from "next/link"
import { Loader2 } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { ModeToggle } from "@/components/mode-toggle"
import { Logo } from "@/components/logo"
import { ExploitResults } from "@/components/exploit-results"
import { MalwareResults } from "@/components/malware-results"
import { api, Scan, ExploitResult, MalwareResult, parseResult } from "@/lib/api"
import { timeAgo } from "@/lib/utils"

function SharedContent() {
  const searchParams = useSearchParams()
  const token = searchParams.get("token") || ""
  const [scan, setScan] = useState<Scan | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")

  useEffect(() => {
    if (!token) {
      setError("No share token specified")
      setLoading(false)
      return
    }
    api.sharedScan(token)
      .then(setScan)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [token])

  if (loading) {
    return <div className="min-h-screen flex items-center justify-center"><Loader2 className="h-5 w-5 animate-spin text-muted-foreground" /></div>
  }

  if (error || !scan) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <p className="text-sm text-muted-foreground">{error || "Scan not found"}</p>
      </div>
    )
  }

  const result = parseResult<ExploitResult | MalwareResult>(scan)

  return (
    <div className="min-h-screen">
      <header className="border-b px-6 h-12 flex items-center justify-between shrink-0">
        <Link href="/" className="flex items-center gap-3">
          <Logo className="h-5 w-auto" />
        </Link>
        <ModeToggle />
      </header>

      <main className="px-6 py-6 max-w-4xl mx-auto space-y-6 animate-fade-in">
        <div className="flex items-center gap-3">
          <Badge variant={scan.type === "exploit" ? "default" : "outline"}>
            {scan.type === "exploit" ? "Exploit Scan" : "Malware Scan"}
          </Badge>
          <Badge variant="outline">shared</Badge>
          {scan.completed_at && (
            <span className="text-xs text-muted-foreground">{timeAgo(scan.completed_at)}</span>
          )}
        </div>

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

        {!result && (
          <p className="text-sm text-muted-foreground">No results available for this scan.</p>
        )}
      </main>
    </div>
  )
}

export default function SharedPage() {
  return (
    <Suspense fallback={<div className="min-h-screen flex items-center justify-center"><Loader2 className="h-5 w-5 animate-spin" /></div>}>
      <SharedContent />
    </Suspense>
  )
}
