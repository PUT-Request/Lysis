"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { Trash2, Search } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Dialog, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Scan, PaginatedScans, api } from "@/lib/api"
import { formatDate } from "@/lib/utils"

function statusVariant(status: string) {
  switch (status) {
    case "completed": return "success" as const
    case "running": return "warning" as const
    case "failed": return "destructive" as const
    default: return "secondary" as const
  }
}

export function ScansTable({
  scans,
  onRefresh,
  search,
  onSearch,
}: {
  scans: PaginatedScans
  onRefresh: (page?: number) => void
  search: string
  onSearch: (query: string) => void
}) {
  const router = useRouter()
  const [page, setPage] = useState(1)
  const [deleteId, setDeleteId] = useState<string | null>(null)

  async function handleDeleteConfirm() {
    if (!deleteId) return
    try {
      await api.deleteScan(deleteId)
    } catch {} finally {
      setDeleteId(null)
      onRefresh()
    }
  }

  if (scans.scans.length === 0) {
    return (
      <div className="border">
        <div className="px-5 py-3 border-b flex items-center justify-between gap-2">
          <h2 className="text-sm font-semibold">Scans</h2>
          <div className="relative max-w-[200px]">
            <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3 w-3 text-muted-foreground" />
            <Input
              placeholder="Search..."
              value={search}
              onChange={(e) => onSearch(e.target.value)}
              className="h-7 pl-6 text-xs"
            />
          </div>
        </div>
        <div className="p-8 text-center text-sm text-muted-foreground">
          {search ? "No scans match your search." : 'No scans yet. Click "New Scan" to start.'}
        </div>
      </div>
    )
  }

  return (
    <div className="border overflow-hidden">
      <div className="px-5 py-3 border-b flex items-center justify-between gap-2">
        <h2 className="text-sm font-semibold">Scans</h2>
        <div className="relative max-w-[200px]">
          <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3 w-3 text-muted-foreground" />
          <Input
            placeholder="Search..."
            value={search}
            onChange={(e) => onSearch(e.target.value)}
            className="h-7 pl-6 text-xs"
          />
        </div>
      </div>
      <div className="overflow-x-auto">
        <Table className="min-w-[640px]">
          <TableHeader>
            <TableRow>
              <TableHead className="w-[130px]">Date</TableHead>
              <TableHead className="w-[140px]">Source</TableHead>
              <TableHead className="w-[90px]">Type</TableHead>
              <TableHead className="w-[90px]">Status</TableHead>
              <TableHead className="w-[90px]">Verdict</TableHead>
              <TableHead className="w-[40px]"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {scans.scans.map((scan, i) => (
              <TableRow
                key={scan.id}
                className="animate-slide-up cursor-pointer"
                style={{ animationDelay: `${i * 40}ms`, animationFillMode: "backwards" }}
                onClick={() => router.push(`/scan?id=${scan.id}`)}
              >
                <TableCell className="text-muted-foreground text-[11px]">
                  {formatDate(scan.created_at)}
                </TableCell>
                <TableCell className="text-[11px] max-w-[140px] truncate">
                  {scan.source_detail || "—"}
                </TableCell>
                <TableCell>
                  <Badge variant="outline" className="text-[10px]">
                    {scan.type === "exploit" ? "Exploit" : "Malware"}
                  </Badge>
                </TableCell>
                <TableCell>
                  <Badge variant={statusVariant(scan.status)} className="text-[10px]">
                    {scan.status}
                  </Badge>
                </TableCell>
                <TableCell className="text-muted-foreground text-[11px]">
                  {scan.verdict || "—"}
                </TableCell>
                <TableCell>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6"
                    onClick={(e) => { e.stopPropagation(); setDeleteId(scan.id); }}
                  >
                    <Trash2 className="h-3 w-3" />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      {scans.total_pages > 1 && (
        <div className="flex items-center justify-center gap-2 p-3 border-t">
          {Array.from({ length: scans.total_pages }, (_, i) => (
            <button
              key={i}
              onClick={() => { setPage(i + 1); onRefresh(i + 1); }}
              className={`h-7 w-7 text-xs border transition-colors ${
                page === i + 1 ? "bg-primary text-primary-foreground" : "hover:bg-muted"
              }`}
            >
              {i + 1}
            </button>
          ))}
        </div>
      )}

      <Dialog open={!!deleteId} onClose={() => setDeleteId(null)}>
        <DialogHeader>
          <DialogTitle>Delete Scan</DialogTitle>
        </DialogHeader>
        <p className="text-xs text-muted-foreground mb-4">
          Permanently delete this scan and all results? This cannot be undone.
        </p>
        <div className="flex justify-end gap-2">
          <Button variant="outline" size="sm" onClick={() => setDeleteId(null)}>
            Cancel
          </Button>
          <Button variant="destructive" size="sm" onClick={handleDeleteConfirm}>
            Delete
          </Button>
        </div>
      </Dialog>
    </div>
  )
}
