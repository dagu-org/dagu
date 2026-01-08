package mail

import "github.com/dagu-org/dagu/internal/core"

// ConfigSchema defines the schema for mail executor config.
// This struct is ONLY for generating JSON Schema - not used at runtime.
type ConfigSchema struct {
	From        string   `json:"from" jsonschema:"Sender email address"`
	To          any      `json:"to" jsonschema:"Recipient email address(es) - string or array of strings"`
	Subject     string   `json:"subject,omitempty" jsonschema:"Email subject line"`
	Message     string   `json:"message,omitempty" jsonschema:"Email body content"`
	Attachments []string `json:"attachments,omitempty" jsonschema:"File paths to attach"`
}

func init() {
	core.RegisterExecutorConfigType[ConfigSchema]("mail")
}
