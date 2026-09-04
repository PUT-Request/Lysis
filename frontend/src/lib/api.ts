const API_BASE = ""

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const token = getToken()
  const isFormData = options?.body instanceof FormData
  const headers: Record<string, string> = {
    ...(isFormData ? {} : { "Content-Type": "application/json" }),
    ...(options?.headers as Record<string, string>),
  }
  if (token) {
    headers["Authorization"] = `Bearer ${token}`
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  })

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: "request failed" }))
    throw new Error(err.error || `HTTP ${res.status}`)
  }

  return res.json()
}

export function getToken(): string | null {
  if (typeof window === "undefined") return null
  const token = localStorage.getItem("lysis_token")
  if (!token) return null
  try {
    const payload = JSON.parse(atob(token.split(".")[1]))
    if (payload.exp && payload.exp * 1000 < Date.now()) {
      localStorage.removeItem("lysis_token")
      return null
    }
  } catch {
    localStorage.removeItem("lysis_token")
    return null
  }
  return token
}

export function setToken(token: string) {
  localStorage.setItem("lysis_token", token)
}

export function clearToken() {
  localStorage.removeItem("lysis_token")
}

export function isAuthenticated(): boolean {
  return !!getToken()
}

export const api = {
  login: (email: string, password: string) =>
    request<{ token: string; user: User }>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),
  signup: (email: string, password: string) =>
    request<{ token: string; user: User }>("/api/auth/signup", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),
  me: () => request<{ user: User }>("/api/auth/me"),
  stats: () => request<Stats>("/api/stats"),
  listScans: (page = 1, search = "") =>
    request<PaginatedScans>(`/api/scans?page=${page}&limit=10${search ? `&search=${encodeURIComponent(search)}` : ""}`),
  getScan: (id: string) => request<Scan>("/api/scans/" + id),
  getScanStatus: (id: string) =>
    request<{ status: string }>("/api/scans/" + id + "/status"),
  createScan: (formData: FormData) =>
    request<{ scan_id: string }>("/api/scans", {
      method: "POST",
      headers: {},
      body: formData,
    }),
  deleteScan: (id: string) =>
    request<{ status: string }>("/api/scans/" + id, { method: "DELETE" }),
  shareScan: (id: string, visibility: string) =>
    request<{ share_token: string; visibility: string }>("/api/scans/" + id + "/share", {
      method: "PATCH",
      body: JSON.stringify({ visibility }),
    }),
  sharedScan: (token: string) => request<Scan>("/api/shared/" + token),
}

export interface User {
  id: number
  email: string
  created_at: string
}

export interface Stats {
  total: number
  completed: number
  running: number
  queued: number
  failed: number
}

export interface Scan {
  id: string
  user_id: number
  type: "exploit" | "malware"
  source: "upload" | "github"
  source_detail?: string
  status: "queued" | "running" | "completed" | "failed"
  verdict?: string
  file_hash?: string
  prescan_json?: string
  result_json?: string
  share_visibility: "private" | "logged_in" | "public"
  share_token?: string
  error_message?: string
  created_at: string
  completed_at?: string
}

export interface PaginatedScans {
  scans: Scan[]
  total: number
  page: number
  total_pages: number
}

export interface ExploitFinding {
  cve_id?: string
  title: string
  severity: "critical" | "high" | "medium" | "low"
  ease_of_exploitation: "easy" | "medium" | "hard"
  cve_type: string
  file_path: string
  line_start?: number
  line_end?: number
  vulnerable_code?: string
  description: string
  attack_vector?: string
  impact?: string
  poc?: string
  remediation?: string
  confidence: number
}

export interface ExploitResult {
  findings: ExploitFinding[]
  risk_score: number
  summary: string
}

export interface BehavioralTimelineEntry {
  timestamp: string
  action: string
  description: string
  file: string
  severity: string
}

export interface RegistryEntry {
  key: string
  value: string
  action: string
  severity: string
}

export interface MalwareFinding {
  file: string
  classification: string
  indicators: string[]
  mitre_attack_ids: string[]
  details: string
}

export interface MalwareResult {
  verdict: string
  confidence: number
  most_likely: string
  behavioral_timeline: BehavioralTimelineEntry[]
  malicious_registry: RegistryEntry[]
  findings: MalwareFinding[]
  skipped_files: string[]
  summary: string
}

export function parseResult<T extends ExploitResult | MalwareResult>(
  scan: Scan
): T | null {
  if (!scan.result_json) return null
  try {
    const parsed = JSON.parse(scan.result_json)
    if (scan.type === "exploit" && Array.isArray(parsed.findings)) {
      return parsed as T
    }
    if (scan.type === "malware" && typeof parsed.verdict === "string") {
      return parsed as T
    }
    return null
  } catch {
    return null
  }
}
