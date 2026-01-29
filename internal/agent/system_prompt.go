package agent

import (
	"bytes"
	"embed"
	"text/template"
)

//go:embed system_prompt.txt
var systemPromptFS embed.FS

// systemPromptTemplate is parsed once at package initialization.
var systemPromptTemplate = template.Must(
	template.New("system_prompt").Parse(mustReadPromptFile()),
)

func mustReadPromptFile() string {
	content, err := systemPromptFS.ReadFile("system_prompt.txt")
	if err != nil {
		panic("failed to read embedded system_prompt.txt: " + err.Error())
	}
	return string(content)
}

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

// GenerateSystemPrompt renders the system prompt template with the given environment
// and optional DAG context.
func GenerateSystemPrompt(env EnvironmentInfo, currentDAG *CurrentDAG) string {
	var buf bytes.Buffer
	data := systemPromptData{
		EnvironmentInfo: env,
		CurrentDAG:      currentDAG,
	}
	if err := systemPromptTemplate.Execute(&buf, data); err != nil {
		return fallbackPrompt(env)
	}
	return buf.String()
}

// fallbackPrompt returns a basic prompt when template execution fails.
func fallbackPrompt(env EnvironmentInfo) string {
	return "You are Hermio, an AI assistant for DAG workflows. DAGs Directory: " + env.DAGsDir
}
