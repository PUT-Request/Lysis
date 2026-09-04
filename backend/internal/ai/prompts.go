package ai

import (
	"fmt"
	"strings"

	"lysis/internal/config"
)

const sandboxInstructions = `You are running inside an isolated sandbox with the following tools available.
You can run commands using these tools to analyze files.

AVAILABLE COMMANDS (run via shell):
{{TOOL_LIST}}

Working directory: /work
All uploaded files are in /work/

You can:
1. Run any of the available tools on files in /input/
2. Read file contents
3. Search for patterns in files

CRITICAL: When you are finished analyzing, call the final_result function with your findings.
Do NOT make up results. Base everything on actual tool output and file contents.`

func BuildExploitPrompt(cfg config.PromptsConfig, availableTools []string) []Message {
	systemPrompt := strings.Replace(cfg.Exploit.SystemPrompt, "{{SANDBOX_INSTRUCTIONS}}", buildSandboxSection(availableTools), 1)
	return []Message{
		{Role: "system", Content: systemPrompt},
	}
}

func BuildMalwarePrompt(cfg config.PromptsConfig, availableTools []string, prescanResults string) []Message {
	instructions := buildSandboxSection(availableTools)
	systemPrompt := cfg.Malware.SystemPrompt
	systemPrompt = strings.Replace(systemPrompt, "{{SANDBOX_INSTRUCTIONS}}", instructions, 1)
	systemPrompt = strings.Replace(systemPrompt, "{{PRESCAN_RESULTS}}", prescanResults, 1)

	return []Message{
		{Role: "system", Content: systemPrompt},
	}
}

func buildSandboxSection(tools []string) string {
	var toolList strings.Builder
	for _, t := range tools {
		toolList.WriteString(fmt.Sprintf("  - %s\n", t))
	}
	return strings.Replace(sandboxInstructions, "{{TOOL_LIST}}", toolList.String(), 1)
}
