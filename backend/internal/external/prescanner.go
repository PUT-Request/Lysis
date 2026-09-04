package external

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"lysis/internal/config"
)

type PrescanResult struct {
	Hash             string             `json:"hash"`
	VTResult         string             `json:"vt_result"`
	MBResult         string             `json:"mb_result"`
	Classification   string             `json:"classification"`
	Entropy          float64            `json:"entropy"`
	SectionEntropy   map[string]float64 `json:"section_entropy,omitempty"`
	Architecture     string             `json:"architecture,omitempty"`
	EntryPoint       string             `json:"entry_point,omitempty"`
	Subsystem        string             `json:"subsystem,omitempty"`
	Imports          []string           `json:"imports,omitempty"`
	Sections         []string           `json:"sections,omitempty"`
	Strings          []string           `json:"strings,omitempty"`
	Base64Found      bool               `json:"base64_found"`
}

func RunPrescanner(ctx context.Context, filePath string, cfg config.Config) (*PrescanResult, error) {
	result := &PrescanResult{}

	hash, err := computeSHA256(filePath)
	if err != nil {
		return nil, fmt.Errorf("compute hash: %w", err)
	}
	result.Hash = hash

	if cfg.ExternalAPIs.AbuseCh.Enabled && cfg.ExternalAPIs.AbuseCh.APIKey != "" {
		mb := NewMBClient(cfg.ExternalAPIs.AbuseCh)
		if mbResult, err := mb.LookupHash(hash); err == nil && mbResult != "" {
			result.MBResult = mbResult
		}
	}

	if result.MBResult == "" && cfg.ExternalAPIs.VirusTotal.Enabled && cfg.ExternalAPIs.VirusTotal.APIKey != "" {
		vt := NewVTClient(cfg.ExternalAPIs.VirusTotal)
		if vtResult, err := vt.LookupFile(hash); err == nil && vtResult != "" {
			result.VTResult = vtResult
		}
	}

	fileType, _ := runQuickCmd(filePath, "file", "-b", filePath)
	if fileType != "" {
		result.Classification = strings.TrimSpace(fileType)
	}

	entropy, _ := computeEntropy(filePath)
	result.Entropy = entropy

	if isPE(fileType) {
		peInfo, _ := runQuickCmd(filePath, "pev", "-t", filePath)
		if peInfo != "" {
			lines := strings.Split(peInfo, "\n")
			for _, line := range lines {
				if strings.Contains(line, "Architecture:") || strings.Contains(line, "Machine:") {
					result.Architecture = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				}
			}
		}

		entryInfo, _ := runQuickCmd(filePath, "pev", "-c", filePath)
		if entryInfo != "" {
			result.EntryPoint = strings.TrimSpace(entryInfo)
		}

		subsysInfo, _ := runQuickCmd(filePath, "pev", "-x", filePath)
		if subsysInfo != "" {
			result.Subsystem = strings.TrimSpace(subsysInfo)
		}

		sectionsInfo, _ := runQuickCmd(filePath, "readelf", "-S", filePath)
		if sectionsInfo == "" {
			sectionsInfo, _ = runQuickCmd(filePath, "llvm-objdump", "-h", filePath)
		}
		if sectionsInfo != "" {
			for _, line := range strings.Split(sectionsInfo, "\n") {
				fields := strings.Fields(line)
				for _, f := range fields {
					if strings.HasPrefix(f, ".") {
						result.Sections = append(result.Sections, f)
					}
				}
			}
		}
	}

	stringsOut, _ := runQuickCmd(filePath, "strings", filePath)
	if stringsOut != "" {
		lines := strings.Split(stringsOut, "\n")
		limit := 100
		if len(lines) < limit {
			limit = len(lines)
		}
		result.Strings = lines[:limit]
		for _, s := range lines {
			if len(s) > 20 && looksLikeBase64(s) {
				result.Base64Found = true
				break
			}
		}
	}

	return result, nil
}

func (p *PrescanResult) ToPromptText() string {
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("=== PRESCAN RESULTS ===\n"))
	b.WriteString(fmt.Sprintf("SHA-256: %s\n", p.Hash))

	if p.MBResult != "" {
		b.WriteString(fmt.Sprintf("\n--- MalwareBazaar ---\nSTATUS: FOUND\n%s\n", p.MBResult))
	} else {
		b.WriteString("\n--- MalwareBazaar ---\nNOT FOUND\n")
	}

	if p.VTResult != "" {
		b.WriteString(fmt.Sprintf("\n--- VirusTotal ---\n%s\n", p.VTResult))
	}

	b.WriteString(fmt.Sprintf("\n--- File Classification ---\n%s\n", p.Classification))
	b.WriteString(fmt.Sprintf("\n--- Entropy ---\n%.2f\n", p.Entropy))

	if p.Architecture != "" {
		b.WriteString(fmt.Sprintf("\n--- Architecture ---\n%s\n", p.Architecture))
	}
	if p.EntryPoint != "" {
		b.WriteString(fmt.Sprintf("\n--- Entry Point ---\n%s\n", p.EntryPoint))
	}
	if p.Subsystem != "" {
		b.WriteString(fmt.Sprintf("\n--- Subsystem ---\n%s\n", p.Subsystem))
	}

	if len(p.Sections) > 0 {
		b.WriteString(fmt.Sprintf("\n--- PE Sections ---\n%s\n", strings.Join(p.Sections, ", ")))
	}

	if len(p.Strings) > 0 {
		b.WriteString(fmt.Sprintf("\n--- Strings (sample) ---\n%s\n", strings.Join(p.Strings[:min(20, len(p.Strings))], "\n")))
	}

	if p.Base64Found {
		b.WriteString("\n--- Base64 Encoded Payloads ---\nDETECTED\n")
	}

	b.WriteString("\n=== END PRESCAN ===\n")
	return b.String()
}

func (p *PrescanResult) JSON() string {
	data, _ := json.Marshal(p)
	return string(data)
}

func computeSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

func runQuickCmd(workDir, cmd string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c := exec.CommandContext(ctx, cmd, args...)
	c.Dir = filepath.Dir(workDir)
	out, err := c.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func computeEntropy(path string) (float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, nil
	}
	counts := make(map[byte]int)
	for _, b := range data {
		counts[b]++
	}
	var entropy float64
	n := float64(len(data))
	for _, c := range counts {
		p := float64(c) / n
		entropy -= p * log2(p)
	}
	return entropy, nil
}

func log2(x float64) float64 {
	return math.Log2(x)
}

func isPE(fileType string) bool {
	return strings.Contains(fileType, "PE32") || strings.Contains(fileType, "MS-DOS")
}

func looksLikeBase64(s string) bool {
	s = strings.TrimRight(s, "=")
	return len(s)%4 == 0 && len(s) > 0
}
