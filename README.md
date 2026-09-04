# Lysis

AI-powered security analysis for codebases. Find exploits and malware with automated sandbox analysis.

## Try It Out

**[lysis.kernal.bid](https://lysis.kernal.bid)** — Free to use, 100 scans per day.

## Features

### Exploit Scanner

Upload a codebase or GitHub repo and get comprehensive security analysis:

- **CVE Mapping** — Findings mapped to known vulnerabilities
- **Attack PoCs** — Actual exploit code, not just descriptions
- **Remediation Steps** — Actionable fixes for each finding
- **Risk Scoring** — 0-100 risk score based on severity and exploitability

Supports: `.zip`, `.py`, `.js`, `.ts`, `.go`, `.cpp`

### Malware Scanner

Analyze binaries and dependencies with static analysis:

- **Verdict & Confidence** — Malicious / suspicious / clean with confidence score
- **Behavioral Timeline** — Step-by-step execution analysis
- **MITRE ATT&CK Mapping** — TTPs mapped to industry framework
- **Registry Forensics** — Persistence mechanisms detected
- **Packers & Obfuscation** — UPX, Themida, VMProtect detection
- **VirusTotal & MalwareBazaar** — Pre-scan hash lookups

Supports: `.exe`, `.dll`, `.bin`

### GitHub Import

Paste a GitHub URL to scan an entire repository. Option to include latest release assets for malware scanning.

### Isolated Sandboxing

All analysis runs in an isolated Linux sandbox (bubblewrap) with:

- Network isolation
- Namespace unsharing
- Command timeout enforcement
- Memory limits
- Auto-cleanup after scan

## Tech Stack

- **Backend:** Go 1.22, SQLite, JWT auth
- **Frontend:** Next.js 16, React 19, Tailwind CSS 4
- **AI:** OpenAI-compatible API with tool-use agent loops
- **Sandbox:** bubblewrap (bwrap) on Linux

## Self-Hosted Setup

### Prerequisites

- Go 1.22+
- Node.js 18+ with pnpm
- Linux (for bubblewrap sandbox)
- An OpenAI-compatible API key

### Backend

```bash
cd backend
cp config.yaml.example config.yaml
# Edit config.yaml with your API keys
go build -o lysis ./cmd/server
./lysis -config config.yaml
```

### Frontend

```bash
cd frontend
pnpm install
pnpm build
```

The static output in `frontend/out` is served by the Go backend.

### Configuration

Copy `backend/config.yaml.example` to `backend/config.yaml` and set:

| Key | Description |
|-----|-------------|
| `llm.api_key` | Your OpenAI-compatible API key |
| `llm.endpoint` | API endpoint URL |
| `auth.jwt_secret` | Secret for JWT token signing |
| `external_apis.virustotal.api_key` | VirusTotal API key (optional) |
| `external_apis.abusech.api_key` | MalwareBazaar API key (optional) |

## License

See [LICENSE](LICENSE) for terms. Non-commercial use only.
