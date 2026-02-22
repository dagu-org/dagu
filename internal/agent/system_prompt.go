package agent

import (
	"bytes"
	_ "embed"
	"text/template"

	"github.com/dagu-org/dagu/internal/auth"
)

//go:embed system_prompt.txt
var systemPromptRaw string

// systemPromptTemplate is parsed once at package initialization.
var systemPromptTemplate = template.Must(
	template.New("system_prompt").Parse(systemPromptRaw),
)

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

// SkillListThreshold is the maximum number of skills to list individually
// in the system prompt. Above this, only the count is shown.
const SkillListThreshold = 20

// systemPromptData contains all data for template rendering.
type systemPromptData struct {
	EnvironmentInfo
	CurrentDAG      *CurrentDAG
	Memory          MemoryContent
	User            *UserCapabilities
	AvailableSkills []SkillSummary
	SkillCount      int
	SoulContent     string
}

// SystemPromptParams holds all parameters for system prompt generation.
type SystemPromptParams struct {
	Env             EnvironmentInfo
	CurrentDAG      *CurrentDAG
	Memory          MemoryContent
	Role            auth.Role
	AvailableSkills []SkillSummary
	SkillCount      int
	Soul            *Soul
}

// GenerateSystemPrompt renders the system prompt template with the given parameters.
func GenerateSystemPrompt(p SystemPromptParams) string {
	env := p.Env
	currentDAG := p.CurrentDAG
	memory := p.Memory
	role := p.Role
	availableSkills := p.AvailableSkills
	skillCount := p.SkillCount
	soul := p.Soul
	var buf bytes.Buffer
	var soulContent string
	if soul != nil {
		soulContent = soul.Content
	}
	data := systemPromptData{
		EnvironmentInfo: env,
		CurrentDAG:      currentDAG,
		Memory:          memory,
		User:            buildUserCapabilities(role),
		AvailableSkills: availableSkills,
		SkillCount:      skillCount,
		SoulContent:     soulContent,
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
	return "You are Dagu Assistant, an AI assistant for DAG workflows. DAGs Directory: " + env.DAGsDir
}
