package api

import (
	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
)

func toDAG(dag *core.DAG) api.DAG {
	schedules := make([]api.Schedule, len(dag.Schedule))
	for i, s := range dag.Schedule {
		schedules[i] = api.Schedule{Expression: s.Expression}
	}

	return api.DAG{
		Name:          dag.Name,
		Group:         ptrOf(dag.Group),
		Description:   ptrOf(dag.Description),
		Params:        ptrOf(dag.Params),
		DefaultParams: ptrOf(dag.DefaultParams),
		Tags:          ptrOf(dag.Tags.Strings()),
		Schedule:      ptrOf(schedules),
	}
}

func toStep(obj core.Step) api.Step {
	conditions := make([]api.Condition, len(obj.Preconditions))
	for i := range obj.Preconditions {
		conditions[i] = toPrecondition(obj.Preconditions[i])
	}

	var repeatMode *api.RepeatMode
	if obj.RepeatPolicy.RepeatMode != "" {
		mode := api.RepeatMode(obj.RepeatPolicy.RepeatMode)
		repeatMode = &mode
	}

	repeatPolicy := api.RepeatPolicy{
		Repeat:   repeatMode,
		Interval: ptrOf(int(obj.RepeatPolicy.Interval.Seconds())),
		Limit:    ptrOf(obj.RepeatPolicy.Limit),
		ExitCode: ptrOf(obj.RepeatPolicy.ExitCode),
	}

	if obj.RepeatPolicy.Condition != nil {
		repeatPolicy.Condition = ptrOf(toPrecondition(obj.RepeatPolicy.Condition))
	}

	commands := make([]api.CommandEntry, len(obj.Commands))
	for i, cmd := range obj.Commands {
		commands[i] = api.CommandEntry{
			Command: cmd.Command,
			Args:    ptrOf(cmd.Args),
		}
	}

	step := api.Step{
		Name:          obj.Name,
		Id:            ptrOf(obj.ID),
		Description:   ptrOf(obj.Description),
		Commands:      ptrOf(commands),
		Depends:       ptrOf(obj.Depends),
		Dir:           ptrOf(obj.Dir),
		MailOnError:   ptrOf(obj.MailOnError),
		Output:        ptrOf(obj.Output),
		Preconditions: ptrOf(conditions),
		RepeatPolicy:  ptrOf(repeatPolicy),
		Script:        ptrOf(obj.Script),
	}

	if obj.Timeout > 0 {
		timeoutSec := int(obj.Timeout.Seconds())
		step.TimeoutSec = &timeoutSec
	}

	if obj.SubDAG != nil {
		step.Call = ptrOf(obj.SubDAG.Name)
		step.Params = ptrOf(obj.SubDAG.Params)
	}
	if obj.Parallel != nil {
		parallel := struct {
			Items         *api.Step_Parallel_Items `json:"items,omitempty"`
			MaxConcurrent *int                     `json:"maxConcurrent,omitempty"`
		}{
			MaxConcurrent: ptrOf(obj.Parallel.MaxConcurrent),
		}

		switch {
		case obj.Parallel.Variable != "":
			items := &api.Step_Parallel_Items{}
			if err := items.FromStepParallelItems1(obj.Parallel.Variable); err == nil {
				parallel.Items = items
			}
		case len(obj.Parallel.Items) > 0:
			itemStrings := make([]string, len(obj.Parallel.Items))
			for i, item := range obj.Parallel.Items {
				itemStrings[i] = item.Value
			}
			items := &api.Step_Parallel_Items{}
			if err := items.FromStepParallelItems0(itemStrings); err == nil {
				parallel.Items = items
			}
		}
		step.Parallel = &parallel
	}

	if obj.ExecutorConfig.Type != "" || obj.ExecutorConfig.Config != nil {
		step.ExecutorConfig = &struct {
			Config *map[string]any `json:"config,omitempty"`
			Type   *string         `json:"type,omitempty"`
		}{
			Type:   ptrOf(obj.ExecutorConfig.Type),
			Config: ptrOf(obj.ExecutorConfig.Config),
		}
	}

	if obj.Router != nil {
		routes := make([]struct {
			Pattern string   `json:"pattern"`
			Targets []string `json:"targets"`
		}, len(obj.Router.Routes))
		for i, r := range obj.Router.Routes {
			routes[i] = struct {
				Pattern string   `json:"pattern"`
				Targets []string `json:"targets"`
			}{
				Pattern: r.Pattern,
				Targets: r.Targets,
			}
		}
		step.Router = &struct {
			Routes []struct {
				Pattern string   `json:"pattern"`
				Targets []string `json:"targets"`
			} `json:"routes"`
			Value string `json:"value"`
		}{
			Value:  obj.Router.Value,
			Routes: routes,
		}
	}

	return step
}

func toPrecondition(obj *core.Condition) api.Condition {
	return api.Condition{
		Condition: obj.Condition,
		Expected:  ptrOf(obj.Expected),
		Negate:    ptrOf(obj.Negate),
		Error:     ptrOf(obj.GetErrorMessage()),
	}
}

func toTriggerType(t core.TriggerType) *api.TriggerType {
	if t == core.TriggerTypeUnknown {
		return nil
	}
	triggerType := api.TriggerType(t.String())
	return &triggerType
}

func toDAGRunSummary(s exec.DAGRunStatus) api.DAGRunSummary {
	return api.DAGRunSummary{
		RootDAGRunName:   s.Root.Name,
		RootDAGRunId:     s.Root.ID,
		ParentDAGRunName: ptrOf(s.Parent.Name),
		ParentDAGRunId:   ptrOf(s.Parent.ID),
		Log:              s.Log,
		Name:             s.Name,
		Params:           ptrOf(s.Params),
		DagRunId:         s.DAGRunID,
		QueuedAt:         ptrOf(s.QueuedAt),
		StartedAt:        s.StartedAt,
		FinishedAt:       s.FinishedAt,
		Status:           api.Status(s.Status),
		StatusLabel:      api.StatusLabel(s.Status.String()),
		WorkerId:         ptrOf(s.WorkerID),
		TriggerType:      toTriggerType(s.TriggerType),
		Tags:             &s.Tags,
	}
}

// ToDAGRunDetails converts a DAGRunStatus to its API representation.
// This function is exported for use by the SSE package.
func ToDAGRunDetails(s exec.DAGRunStatus) api.DAGRunDetails {
	preconditions := make([]api.Condition, len(s.Preconditions))
	for i, p := range s.Preconditions {
		preconditions[i] = toPrecondition(p)
	}
	nodes := make([]api.Node, len(s.Nodes))
	for i, n := range s.Nodes {
		nodes[i] = toNode(n)
	}

	return api.DAGRunDetails{
		RootDAGRunName:   s.Root.Name,
		RootDAGRunId:     s.Root.ID,
		ParentDAGRunName: ptrOf(s.Parent.Name),
		ParentDAGRunId:   ptrOf(s.Parent.ID),
		Log:              s.Log,
		Name:             s.Name,
		Params:           ptrOf(s.Params),
		DagRunId:         s.DAGRunID,
		StartedAt:        s.StartedAt,
		FinishedAt:       s.FinishedAt,
		Status:           api.Status(s.Status),
		StatusLabel:      api.StatusLabel(s.Status.String()),
		WorkerId:         ptrOf(s.WorkerID),
		TriggerType:      toTriggerType(s.TriggerType),
		Preconditions:    ptrOf(preconditions),
		Nodes:            nodes,
		OnSuccess:        ptrOf(toNode(s.OnSuccess)),
		OnFailure:        ptrOf(toNode(s.OnFailure)),
		OnCancel:         ptrOf(toNode(s.OnCancel)),
		OnExit:           ptrOf(toNode(s.OnExit)),
		Tags:             &s.Tags,
	}
}

func toNode(node *exec.Node) api.Node {
	if node == nil {
		return api.Node{}
	}
	return api.Node{
		DoneCount:       node.DoneCount,
		FinishedAt:      node.FinishedAt,
		Stdout:          node.Stdout,
		Stderr:          node.Stderr,
		RetryCount:      node.RetryCount,
		StartedAt:       node.StartedAt,
		Status:          api.NodeStatus(node.Status),
		StatusLabel:     api.NodeStatusLabel(node.Status.String()),
		Step:            toStep(node.Step),
		Error:           ptrOf(node.Error),
		SubRuns:         ptrOf(toSubDAGRuns(node.SubRuns)),
		SubRunsRepeated: ptrOf(toSubDAGRuns(node.SubRunsRepeated)),
		ApprovedAt:      ptrOf(node.ApprovedAt),
		ApprovedBy:      ptrOf(node.ApprovedBy),
		ApprovalInputs:  ptrOf(node.ApprovalInputs),
		RejectedAt:      ptrOf(node.RejectedAt),
		RejectedBy:      ptrOf(node.RejectedBy),
		RejectionReason: ptrOf(node.RejectionReason),
	}
}

func toSubDAGRuns(subDAGRuns []exec.SubDAGRun) []api.SubDAGRun {
	result := make([]api.SubDAGRun, len(subDAGRuns))
	for i, w := range subDAGRuns {
		result[i] = api.SubDAGRun{
			DagRunId: w.DAGRunID,
			Params:   ptrOf(w.Params),
			DagName:  ptrOf(w.DAGName),
		}
	}
	return result
}

func toLocalDAG(dag *core.DAG) api.LocalDag {
	return api.LocalDag{
		Name:   dag.Name,
		Dag:    toDAGDetails(dag),
		Errors: []string{},
	}
}

func toDAGDetails(dag *core.DAG) *api.DAGDetails {
	if dag == nil {
		return nil
	}

	steps := make([]api.Step, len(dag.Steps))
	for i, step := range dag.Steps {
		steps[i] = toStep(step)
	}

	handlerOn := toHandlerOn(dag.HandlerOn)

	schedules := make([]api.Schedule, len(dag.Schedule))
	for i, s := range dag.Schedule {
		schedules[i] = api.Schedule{Expression: s.Expression}
	}

	preconditions := make([]api.Condition, len(dag.Preconditions))
	for i, p := range dag.Preconditions {
		preconditions[i] = toPrecondition(p)
	}

	var runConfig *api.RunConfig
	if dag.RunConfig != nil {
		runConfig = &api.RunConfig{
			DisableParamEdit: dag.RunConfig.DisableParamEdit,
			DisableRunIdEdit: dag.RunConfig.DisableRunIdEdit,
		}
	}

	return &api.DAGDetails{
		Name:              dag.Name,
		Description:       ptrOf(dag.Description),
		DefaultParams:     ptrOf(dag.DefaultParams),
		Delay:             ptrOf(int(dag.Delay.Seconds())),
		Env:               ptrOf(dag.Env),
		Group:             ptrOf(dag.Group),
		HandlerOn:         ptrOf(handlerOn),
		HistRetentionDays: ptrOf(dag.HistRetentionDays),
		LogDir:            ptrOf(dag.LogDir),
		MaxActiveRuns:     ptrOf(dag.MaxActiveRuns),
		MaxActiveSteps:    ptrOf(dag.MaxActiveSteps),
		Params:            ptrOf(dag.Params),
		Preconditions:     ptrOf(preconditions),
		Schedule:          ptrOf(schedules),
		Steps:             ptrOf(steps),
		Tags:              ptrOf(dag.Tags.Strings()),
		RunConfig:         runConfig,
	}
}

func toHandlerOn(handlers core.HandlerOn) api.HandlerOn {
	handlerOn := api.HandlerOn{}
	if handlers.Failure != nil {
		handlerOn.Failure = ptrOf(toStep(*handlers.Failure))
	}
	if handlers.Success != nil {
		handlerOn.Success = ptrOf(toStep(*handlers.Success))
	}
	if handlers.Cancel != nil {
		handlerOn.Cancel = ptrOf(toStep(*handlers.Cancel))
	}
	if handlers.Exit != nil {
		handlerOn.Exit = ptrOf(toStep(*handlers.Exit))
	}
	return handlerOn
}

func toChatMessages(messages []exec.LLMMessage) []api.ChatMessage {
	if messages == nil {
		return []api.ChatMessage{}
	}

	result := make([]api.ChatMessage, len(messages))
	for i, msg := range messages {
		result[i] = toChatMessage(msg)
	}
	return result
}

func toChatMessage(msg exec.LLMMessage) api.ChatMessage {
	apiMsg := api.ChatMessage{
		Role:    api.ChatMessageRole(msg.Role),
		Content: msg.Content,
	}

	if len(msg.ToolCalls) > 0 {
		toolCalls := make([]api.ChatToolCall, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			toolCalls[i] = api.ChatToolCall{
				Id:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: ptrOf(tc.Function.Arguments),
			}
		}
		apiMsg.ToolCalls = &toolCalls
	}

	if msg.Metadata != nil {
		apiMsg.Metadata = &api.ChatMessageMetadata{
			Provider:         ptrOf(msg.Metadata.Provider),
			Model:            ptrOf(msg.Metadata.Model),
			PromptTokens:     ptrOf(msg.Metadata.PromptTokens),
			CompletionTokens: ptrOf(msg.Metadata.CompletionTokens),
			TotalTokens:      ptrOf(msg.Metadata.TotalTokens),
		}
	}

	return apiMsg
}

func toToolDefinitions(defs []exec.ToolDefinition) *[]api.ToolDefinition {
	if len(defs) == 0 {
		return nil
	}

	result := make([]api.ToolDefinition, len(defs))
	for i, def := range defs {
		result[i] = api.ToolDefinition{
			Name:        def.Name,
			Description: ptrOf(def.Description),
			Parameters:  ptrOf(def.Parameters),
		}
	}

	return &result
}
