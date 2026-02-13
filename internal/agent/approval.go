package agent

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
)

func requestCommandApprovalWithOptions(
	parentCtx context.Context,
	emit EmitUserPromptFunc,
	wait WaitUserResponseFunc,
	cmd string,
	workingDir string,
	question string,
	allowIfUnavailable bool,
) (bool, error) {
	if emit == nil || wait == nil {
		if allowIfUnavailable {
			return true, nil
		}
		return false, errors.New("approval channel unavailable")
	}

	promptID := uuid.New().String()
	emit(UserPrompt{
		PromptID:   promptID,
		PromptType: PromptTypeCommandApproval,
		Question:   question,
		Command:    cmd,
		WorkingDir: workingDir,
		Options: []UserPromptOption{
			{ID: "approve", Label: "Approve"},
			{ID: "reject", Label: "Reject"},
		},
	})

	if parentCtx == nil {
		parentCtx = context.Background()
	}
	timeoutCtx, cancel := context.WithTimeout(parentCtx, approvalTimeout)
	defer cancel()

	resp, err := wait(timeoutCtx, promptID)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return false, fmt.Errorf("approval timed out after %v", approvalTimeout)
		}
		return false, err
	}
	if resp.Cancelled {
		return false, nil
	}
	return slices.Contains(resp.SelectedOptionIDs, "approve"), nil
}
