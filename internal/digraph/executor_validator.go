package digraph

import "fmt"

// ExecutorValidator validates executor-specific configurations
type ExecutorValidator interface {
	ValidateStep(step *Step) error
}

// ExecutorValidatorRegistry maps executor types to their validators
var executorValidatorRegistry = make(map[string]ExecutorValidator)

// RegisterExecutorValidator registers a validator for an executor type
func RegisterExecutorValidator(executorType string, validator ExecutorValidator) {
	executorValidatorRegistry[executorType] = validator
}

// SSHExecutorValidator validates SSH executor configurations
type SSHExecutorValidator struct{}

func (v *SSHExecutorValidator) ValidateStep(step *Step) error {
	// SSH executor doesn't support script field
	if step.Script != "" {
		return fmt.Errorf(
			"script field is not supported with SSH executor. " +
				"Use 'command' field instead. " +
				"See: https://github.com/dagu-org/dagu/issues/1306",
		)
	}
	return nil
}

// DockerExecutorValidator validates Docker executor configurations
type DockerExecutorValidator struct{}

func (v *DockerExecutorValidator) ValidateStep(step *Step) error {
	// Docker supports both command and script
	// Add Docker-specific validations here in the future if needed
	return nil
}

// HTTPExecutorValidator validates HTTP executor configurations
type HTTPExecutorValidator struct{}

func (v *HTTPExecutorValidator) ValidateStep(step *Step) error {
	// HTTP-specific validations can be added here
	return nil
}

// MailExecutorValidator validates Mail executor configurations
type MailExecutorValidator struct{}

func (v *MailExecutorValidator) ValidateStep(step *Step) error {
	// Mail-specific validations can be added here
	return nil
}

// JQExecutorValidator validates JQ executor configurations
type JQExecutorValidator struct{}

func (v *JQExecutorValidator) ValidateStep(step *Step) error {
	// JQ-specific validations can be added here
	return nil
}

func init() {
	// Register validators for each executor type
	RegisterExecutorValidator("ssh", &SSHExecutorValidator{})
	RegisterExecutorValidator("docker", &DockerExecutorValidator{})
	RegisterExecutorValidator("http", &HTTPExecutorValidator{})
	RegisterExecutorValidator("mail", &MailExecutorValidator{})
	RegisterExecutorValidator("jq", &JQExecutorValidator{})
}
