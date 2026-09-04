"use client"

import { useState, useRef } from "react"
import { Upload, GitFork, Loader2, File, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input, Label } from "@/components/ui/input"
import { Dialog, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { api } from "@/lib/api"

export function NewScanDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean
  onClose: () => void
  onCreated: () => void
}) {
  const [tab, setTab] = useState<"upload" | "github">("upload")
  const [scanType, setScanType] = useState<"exploit" | "malware">("exploit")
  const [error, setError] = useState("")
  const [uploading, setUploading] = useState(false)
  const [fileObj, setFileObj] = useState<File | null>(null)
  const [fileName, setFileName] = useState("")
  const [githubUrl, setGithubUrl] = useState("")
  const [includeReleases, setIncludeReleases] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  function handleFileDrop(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0] || null
    setFileObj(file)
    setFileName(file ? file.name : "")
  }

  function clearFile() {
    setFileObj(null)
    setFileName("")
    if (fileRef.current) fileRef.current.value = ""
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError("")

    if (tab === "upload") {
      if (!fileObj) {
        setError("Select a file")
        return
      }

      const maxBytes = scanType === "exploit" ? 50 * 1024 * 1024 : 200 * 1024 * 1024
      if (fileObj.size > maxBytes) {
        setError(`File exceeds maximum size (${scanType === "exploit" ? "50" : "200"}MB)`)
        return
      }

      const allowedExts = [".zip", ".py", ".js", ".ts", ".go", ".cpp", ".exe", ".dll", ".bin"]
      const ext = "." + fileObj.name.split(".").pop()?.toLowerCase()
      if (!allowedExts.includes(ext)) {
        setError(`File type .${ext} not supported`)
        return
      }
      setUploading(true)
      try {
        const fd = new FormData()
        fd.append("file", fileObj)
        fd.append("type", scanType)
        await api.createScan(fd)
        onCreated()
        onClose()
        resetForm()
      } catch (err: any) {
        setError(err.message)
      } finally {
        setUploading(false)
      }
    } else {
      if (!githubUrl.trim()) {
        setError("Enter a GitHub URL")
        return
      }
      setUploading(true)
      try {
        const res = await fetch("/api/scans/github", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${localStorage.getItem("lysis_token")}`,
          },
          body: JSON.stringify({
            url: githubUrl,
            type: scanType,
            include_releases: scanType === "malware" && includeReleases,
          }),
        })
        const data = await res.json()
        if (!res.ok) throw new Error(data.error || "github import failed")
        onCreated()
        onClose()
        resetForm()
      } catch (err: any) {
        setError(err.message)
      } finally {
        setUploading(false)
      }
    }
  }

  function resetForm() {
    setFileObj(null)
    setFileName("")
    setGithubUrl("")
    setIncludeReleases(false)
    setTab("upload")
    setScanType("exploit")
  }

  return (
    <Dialog open={open} onClose={onClose}>
      <DialogHeader>
        <DialogTitle>New Scan</DialogTitle>
      </DialogHeader>

      <div className="flex gap-1.5 mb-4">
        <button
          type="button"
          onClick={() => setScanType("exploit")}
          className={`flex-1 border p-2 text-center text-xs font-medium transition-colors ${
            scanType === "exploit"
              ? "border-foreground bg-muted"
              : "hover:bg-muted/50"
          }`}
        >
          Exploit Scan
        </button>
        <button
          type="button"
          onClick={() => setScanType("malware")}
          className={`flex-1 border p-2 text-center text-xs font-medium transition-colors ${
            scanType === "malware"
              ? "border-foreground bg-muted"
              : "hover:bg-muted/50"
          }`}
        >
          Malware Scan
        </button>
      </div>

      <div className="flex border-b border-muted mb-4">
        {(["upload", "github"] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`flex items-center gap-1.5 px-3 py-2 text-xs transition-colors border-b-2 -mb-px ${
              tab === t
                ? "border-foreground text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            {t === "upload" ? (
              <Upload className="h-3 w-3" />
            ) : (
              <GitFork className="h-3 w-3" />
            )}
            {t === "upload" ? "File Upload" : "GitHub Repo"}
          </button>
        ))}
      </div>

      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="min-h-[150px]">
        {tab === "upload" ? (
          <div className="space-y-3">
            <div
              onClick={() => fileRef.current?.click()}
              className={`border p-6 text-center cursor-pointer transition-colors ${
                fileName
                  ? "border-solid border-foreground/20 bg-muted/30"
                  : "border-dashed hover:bg-muted/50"
              }`}
            >
              {fileName ? (
                <File className="h-5 w-5 mx-auto mb-2 text-foreground" />
              ) : (
                <Upload className="h-5 w-5 mx-auto mb-2 text-muted-foreground" />
              )}
              {fileName ? (
                <div className="flex items-center justify-center gap-2">
                  <p className="text-xs font-medium truncate max-w-[200px]">{fileName}</p>
                  <button
                    type="button"
                    onClick={(e) => { e.stopPropagation(); clearFile(); }}
                    className="shrink-0 p-0.5 hover:bg-muted transition-colors"
                  >
                    <X className="h-3 w-3 text-muted-foreground" />
                  </button>
                </div>
              ) : (
                <>
                  <p className="text-xs text-muted-foreground">
                    Click to browse or drop a file here
                  </p>
                  <p className="text-[10px] text-muted-foreground mt-1">
                    .zip .py .js .ts .go .cpp .exe .dll .bin
                  </p>
                </>
              )}
              <input
                ref={fileRef}
                type="file"
                className="hidden"
                accept=".zip,.py,.js,.ts,.go,.cpp,.exe,.dll,.bin"
                onChange={handleFileDrop}
              />
            </div>
            <p className="text-[10px] text-muted-foreground">
              Max {scanType === "exploit" ? "50" : "200"}MB uncompressed
            </p>
          </div>
        ) : (
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="github-url">Repository URL</Label>
              <Input
                id="github-url"
                placeholder="https://github.com/user/repo"
                value={githubUrl}
                onChange={(e) => setGithubUrl(e.target.value)}
              />
            </div>
            {scanType === "malware" && (
              <label className="flex items-center gap-2 cursor-pointer group">
                <input
                  type="checkbox"
                  checked={includeReleases}
                  onChange={(e) => setIncludeReleases(e.target.checked)}
                  className="h-3.5 w-3.5"
                />
                <span className="text-xs text-muted-foreground group-hover:text-foreground transition-colors">
                  Include latest release assets
                </span>
              </label>
            )}
          </div>
        )}
        </div>

        {error && (
          <div className="border border-destructive p-2.5">
            <p className="text-xs text-destructive">{error}</p>
          </div>
        )}

        <div className="flex justify-end gap-2 pt-1">
          <Button variant="outline" type="button" onClick={onClose} size="sm">
            Cancel
          </Button>
          <Button type="submit" disabled={uploading} size="sm">
            {uploading ? (
              <>
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                Scanning...
              </>
            ) : (
              <>
                <Upload className="h-3.5 w-3.5" />
                Start Scan
              </>
            )}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
