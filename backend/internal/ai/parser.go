package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"gopkg.in/yaml.v3"

	"lysis/internal/config"
)

type ExploitResult struct {
	Findings  []ExploitFinding `json:"findings" yaml:"findings"`
	RiskScore int              `json:"risk_score" yaml:"risk_score"`
	Summary   string           `json:"summary" yaml:"summary"`
}

type ExploitFinding struct {
	CVEID            string `json:"cve_id" yaml:"cve_id"`
	Title            string `json:"title" yaml:"title"`
	Severity         string `json:"severity" yaml:"severity"`
	EaseOfExploitation string `json:"ease_of_exploitation" yaml:"ease_of_exploitation"`
	CVEType          string `json:"cve_type" yaml:"cve_type"`
	FilePath         string `json:"file_path" yaml:"file_path"`
	LineStart        *int   `json:"line_start" yaml:"line_start"`
	LineEnd          *int   `json:"line_end" yaml:"line_end"`
	VulnerableCode   string `json:"vulnerable_code" yaml:"vulnerable_code"`
	Description      string `json:"description" yaml:"description"`
	AttackVector     string `json:"attack_vector" yaml:"attack_vector"`
	Impact           string `json:"impact" yaml:"impact"`
	PoC              string `json:"poc" yaml:"poc"`
	Remediation      string `json:"remediation" yaml:"remediation"`
	Confidence       int    `json:"confidence" yaml:"confidence"`
}

type MalwareResult struct {
	Verdict            string                   `json:"verdict" yaml:"verdict"`
	Confidence         int                      `json:"confidence" yaml:"confidence"`
	MostLikely         string                   `json:"most_likely" yaml:"most_likely"`
	BehavioralTimeline []BehavioralTimelineEntry `json:"behavioral_timeline" yaml:"behavioral_timeline"`
	MaliciousRegistry  []RegistryEntry          `json:"malicious_registry" yaml:"malicious_registry"`
	Findings            []MalwareFinding         `json:"findings" yaml:"findings"`
	SkippedFiles       []string                 `json:"skipped_files" yaml:"skipped_files"`
	Summary            string                   `json:"summary" yaml:"summary"`
}

type BehavioralTimelineEntry struct {
	Timestamp   string `json:"timestamp" yaml:"timestamp"`
	Action      string `json:"action" yaml:"action"`
	Description string `json:"description" yaml:"description"`
	File        string `json:"file" yaml:"file"`
	Severity    string `json:"severity" yaml:"severity"`
}

type RegistryEntry struct {
	Key      string `json:"key" yaml:"key"`
	Value    string `json:"value" yaml:"value"`
	Action   string `json:"action" yaml:"action"`
	Severity string `json:"severity" yaml:"severity"`
}

type MalwareFinding struct {
	File           string   `json:"file" yaml:"file"`
	Classification string   `json:"classification" yaml:"classification"`
	Indicators     []string `json:"indicators" yaml:"indicators"`
	MitreAttackIDs []string `json:"mitre_attack_ids" yaml:"mitre_attack_ids"`
	Details        string   `json:"details" yaml:"details"`
}

func ParseExploitYAML(raw string) (*ExploitResult, error) {
	raw = cleanYAMLLevel(raw, 0)
	var result ExploitResult
	if err := yaml.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("invalid exploit YAML: %w", err)
	}
	if len(result.Findings) == 0 {
		return nil, fmt.Errorf("no findings in result")
	}
	return &result, nil
}

func ParseMalwareYAML(raw string) (*MalwareResult, error) {
	raw = cleanYAMLLevel(raw, 0)
	var result MalwareResult
	if err := yaml.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("invalid malware YAML: %w", err)
	}
	if result.Verdict == "" {
		return nil, fmt.Errorf("missing required field: verdict")
	}
	if result.Confidence <= 0 {
		return nil, fmt.Errorf("missing required field: confidence")
	}
	return &result, nil
}

func ValidateMalwareResult(r *MalwareResult) error {
	if r.Verdict == "" {
		return fmt.Errorf("missing required field: verdict")
	}
	if r.Confidence <= 0 {
		return fmt.Errorf("missing required field: confidence")
	}
	return nil
}

func ValidateExploitResult(r *ExploitResult) error {
	if len(r.Findings) == 0 {
		return fmt.Errorf("missing required field: findings")
	}
	return nil
}

func ParseWithRetry(ctx context.Context, client ChatClient, scanType string, messages []Message, cfg config.LLMConfig) (json.RawMessage, error) {
	lastMsg := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" || messages[i].Content != "" {
			lastMsg = messages[i].Content
			break
		}
	}
	if lastMsg == "" {
		return nil, fmt.Errorf("no YAML content to parse")
	}

	raw := extractYAMLFromMessage(lastMsg)
	if raw == "" {
		return nil, fmt.Errorf("no YAML found in message")
	}

	var lastErr error
	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		cleaned := cleanYAMLLevel(raw, attempt)

		if scanType == "exploit" {
			result, err := ParseExploitYAML(cleaned)
			if err == nil {
				jsonBytes, _ := json.Marshal(result)
				return jsonBytes, nil
			}
			lastErr = err
		} else {
			result, err := ParseMalwareYAML(cleaned)
			if err == nil {
				jsonBytes, _ := json.Marshal(result)
				return jsonBytes, nil
			}
			lastErr = err
		}

		log.Printf("YAML parse attempt %d failed: %v", attempt+1, lastErr)
	}

	return nil, fmt.Errorf("failed to parse YAML after %d attempts: %w", cfg.MaxRetries+1, lastErr)
}

func extractYAMLFromMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	start := strings.Index(msg, "findings:")
	if start == -1 {
		start = strings.Index(msg, "verdict:")
	}
	if start == -1 {
		start = strings.Index(msg, "risk_score:")
	}
	if start == -1 {
		return msg
	}
	return msg[start:]
}

func cleanYAMLLevel(raw string, level int) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```yaml")
	raw = strings.TrimPrefix(raw, "```yml")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	if level >= 1 {
		lines := strings.Split(raw, "\n")
		cleaned := make([]string, 0, len(lines))
		for _, line := range lines {
			trimmed := strings.TrimLeft(line, " \t")
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			cleaned = append(cleaned, line)
		}
		raw = strings.Join(cleaned, "\n")
	}

	if level >= 2 {
		raw = strings.Map(func(r rune) rune {
			if r == '\t' {
				return ' '
			}
			if r < 32 && r != '\n' && r != '\r' {
				return -1
			}
			return r
		}, raw)
	}

	if level >= 3 {
		raw = strings.ReplaceAll(raw, "\r\n", "\n")
		for strings.Contains(raw, "\n\n\n") {
			raw = strings.ReplaceAll(raw, "\n\n\n", "\n\n")
		}
	}

	return strings.TrimSpace(raw)
}
