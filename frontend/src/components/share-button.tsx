"use client"

import { useState } from "react"
import { Share2, Check, Copy, Globe, Lock, Users, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Scan, api } from "@/lib/api"
import { getBaseUrl } from "@/lib/utils"

export function ShareButton({ scan }: { scan: Scan }) {
  const [open, setOpen] = useState(false)
  const [visibility, setVisibility] = useState(scan.share_visibility)
  const [shareToken, setShareToken] = useState(scan.share_token || "")
  const [copied, setCopied] = useState(false)
  const [updating, setUpdating] = useState(false)

  async function handleChange(v: "private" | "logged_in" | "public") {
    setVisibility(v)
    if (v === "private") setShareToken("")
    setUpdating(true)
    try {
      const res = await api.shareScan(scan.id, v)
      if (res.share_token) setShareToken(res.share_token)
    } catch {
      setVisibility(scan.share_visibility)
      if (scan.share_token) setShareToken(scan.share_token)
    } finally {
      setUpdating(false)
    }
  }

  async function handleCopy() {
    try {
      const url = `${getBaseUrl()}/shared?token=${shareToken}`
      await navigator.clipboard.writeText(url)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {}
  }

  const options: { id: string; label: string; desc: string; icon: React.ComponentType<{ className?: string }> }[] = [
    { id: "private", label: "Private", desc: "Only you", icon: Lock },
    { id: "logged_in", label: "Logged in", desc: "Any authenticated user", icon: Users },
    { id: "public", label: "Public", desc: "Anyone with the link", icon: Globe },
  ]

  return (
    <>
      <Button variant="ghost" size="sm" onClick={() => setOpen(true)}>
        <Share2 className="h-3.5 w-3.5" />
        Share
      </Button>

      <Dialog open={open} onClose={() => setOpen(false)}>
        <DialogHeader>
          <DialogTitle>Share Scan</DialogTitle>
        </DialogHeader>
        <div className="space-y-3">
          {options.map((opt) => (
            <button
              key={opt.id}
              onClick={() => handleChange(opt.id as "private" | "logged_in" | "public")}
              disabled={updating}
              className={`w-full border p-3 flex items-center gap-3 text-left transition-all duration-150 ${
                visibility === opt.id
                  ? "border-foreground bg-muted"
                  : "hover:bg-muted/50"
              } ${updating && visibility === opt.id ? "opacity-60" : ""}`}
            >
              <opt.icon className="h-4 w-4 text-muted-foreground shrink-0" />
              <div className="flex-1">
                <p className="text-xs font-medium">{opt.label}</p>
                <p className="text-[10px] text-muted-foreground">{opt.desc}</p>
              </div>
              {updating && visibility === opt.id && (
                <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" />
              )}
            </button>
          ))}

          {visibility !== "private" && shareToken && (
            <div className="border p-3 space-y-2 animate-slide-up">
              <p className="text-xs text-muted-foreground">Share link</p>
              <div className="flex items-center gap-2">
                <code className="flex-1 border px-2 py-1.5 text-xs truncate bg-muted/50">
                  {getBaseUrl()}/shared?token={shareToken}
                </code>
                <Button size="sm" variant="outline" onClick={handleCopy}>
                  {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                </Button>
              </div>
            </div>
          )}
        </div>
      </Dialog>
    </>
  )
}
