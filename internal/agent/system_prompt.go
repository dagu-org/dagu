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

// SystemPromptData contains all data for template rendering.
type SystemPromptData struct {
	WorkingDir string
	DAGsDir    string
	LogDir     string
	DataDir    string
	ConfigFile string
	CurrentDAG *CurrentDAG
}

// GenerateSystemPrompt renders the system prompt template.
func GenerateSystemPrompt(env EnvironmentInfo, currentDAG *CurrentDAG) string {
	tmplContent, err := systemPromptFS.ReadFile("system_prompt.txt")
	if err != nil {
		return fallbackPrompt(env)
	}

	tmpl, err := template.New("system_prompt").Parse(string(tmplContent))
	if err != nil {
		return fallbackPrompt(env)
	}

	data := SystemPromptData{
		WorkingDir: env.WorkingDir,
		DAGsDir:    env.DAGsDir,
		LogDir:     env.LogDir,
		DataDir:    env.DataDir,
		ConfigFile: env.ConfigFile,
		CurrentDAG: currentDAG,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fallbackPrompt(env)
	}

	return buf.String()
}

// fallbackPrompt returns a basic prompt when template fails.
func fallbackPrompt(env EnvironmentInfo) string {
	return "You are Hermio, an AI assistant for DAG workflows. " +
		"DAGs Directory: " + env.DAGsDir
}
