package agent

import (
	"bytes"
	"embed"
	"text/template"
)

//go:embed system_prompt.txt
var systemPromptFS embed.FS

// CurrentDAG contains context about the DAG being viewed.
type CurrentDAG struct {
	Name     string
	FilePath string
	RunID    string
	Status   string
}

// systemPromptData contains all data for template rendering.
type systemPromptData struct {
	EnvironmentInfo
	CurrentDAG *CurrentDAG
}

// GenerateSystemPrompt renders the system prompt template.
func GenerateSystemPrompt(env EnvironmentInfo, currentDAG *CurrentDAG) string {
	content, err := systemPromptFS.ReadFile("system_prompt.txt")
	if err != nil {
		return fallbackPrompt(env)
	}

	tmpl, err := template.New("system_prompt").Parse(string(content))
	if err != nil {
		return fallbackPrompt(env)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, systemPromptData{env, currentDAG}); err != nil {
		return fallbackPrompt(env)
	}

	return buf.String()
}

// fallbackPrompt returns a basic prompt when template fails.
func fallbackPrompt(env EnvironmentInfo) string {
	return "You are Hermio, an AI assistant for DAG workflows. DAGs Directory: " + env.DAGsDir
}
