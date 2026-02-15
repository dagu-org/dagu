package agent

import (
	"bytes"
	"embed"
	"text/template"

	"github.com/dagu-org/dagu/internal/agent/iface"
	"github.com/dagu-org/dagu/internal/auth"
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

// UserCapabilities contains role and capability context for the current user.
type UserCapabilities struct {
	Role           string
	CanExecuteDAGs bool
	CanWriteDAGs   bool
	CanViewAudit   bool
	IsAdmin        bool
}

// systemPromptData contains all data for template rendering.
type systemPromptData struct {
	EnvironmentInfo
	CurrentDAG *CurrentDAG
	Memory     iface.MemoryContent
	User       *UserCapabilities
}

// GenerateSystemPrompt renders the system prompt template with the given environment,
// optional DAG context, memory content, and user role capabilities.
func GenerateSystemPrompt(env EnvironmentInfo, currentDAG *CurrentDAG, memory iface.MemoryContent, role auth.Role) string {
	var buf bytes.Buffer
	data := systemPromptData{
		EnvironmentInfo: env,
		CurrentDAG:      currentDAG,
		Memory:          memory,
		User:            buildUserCapabilities(role),
	}
	if err := systemPromptTemplate.Execute(&buf, data); err != nil {
		return fallbackPrompt(env)
	}
	return buf.String()
}

func buildUserCapabilities(role auth.Role) *UserCapabilities {
	if role == "" {
		return nil
	}
	return &UserCapabilities{
		Role:           role.String(),
		CanExecuteDAGs: role.CanExecute(),
		CanWriteDAGs:   role.CanWrite(),
		CanViewAudit:   role.CanManageAudit(),
		IsAdmin:        role.IsAdmin(),
	}
}

// fallbackPrompt returns a basic prompt when template execution fails.
func fallbackPrompt(env EnvironmentInfo) string {
	return "You are Tsumugi, an AI assistant for DAG workflows. DAGs Directory: " + env.DAGsDir
}
