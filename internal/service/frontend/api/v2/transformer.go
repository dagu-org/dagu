package api

import (
	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
)

func toDAG(dag *core.DAG) api.DAG {
	var schedules []api.Schedule
	for _, s := range dag.Schedule {
		schedules = append(schedules, api.Schedule{Expression: s.Expression})
	}

	return api.DAG{
		Name:          dag.Name,
		Group:         ptrOf(dag.Group),
		Description:   ptrOf(dag.Description),
		Params:        ptrOf(dag.Params),
		DefaultParams: ptrOf(dag.DefaultParams),
		Tags:          ptrOf(dag.Tags),
		Schedule:      ptrOf(schedules),
	}
}

func toStep(obj core.Step) api.Step {
	var conditions []api.Condition
	for i := range obj.Preconditions {
		conditions = append(conditions, toPrecondition(obj.Preconditions[i]))
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

	// Convert Commands to API format
	var commands []api.CommandEntry
	for _, cmd := range obj.Commands {
		commands = append(commands, api.CommandEntry{
			Command: cmd.Command,
			Args:    ptrOf(cmd.Args),
		})
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

	// Convert timeout duration to seconds if set
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

		if obj.Parallel.Variable != "" {
			// Variable reference (string)
			items := &api.Step_Parallel_Items{}
			if err := items.FromStepParallelItems1(obj.Parallel.Variable); err == nil {
				parallel.Items = items
			}
		} else if len(obj.Parallel.Items) > 0 {
			// Convert items to string array
			var itemStrings []string
			for _, item := range obj.Parallel.Items {
				itemStrings = append(itemStrings, item.Value)
			}
			// Array of strings
			items := &api.Step_Parallel_Items{}
			if err := items.FromStepParallelItems0(itemStrings); err == nil {
				parallel.Items = items
			}
		}
		step.Parallel = &parallel
	}

	// Add executor config if present
	if obj.ExecutorConfig.Type != "" || obj.ExecutorConfig.Config != nil {
		step.ExecutorConfig = &struct {
			Config *map[string]interface{} `json:"config,omitempty"`
			Type   *string                 `json:"type,omitempty"`
		}{
			Type:   ptrOf(obj.ExecutorConfig.Type),
			Config: ptrOf(obj.ExecutorConfig.Config),
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

func toDAGRunSummary(s execution.DAGRunStatus) api.DAGRunSummary {
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
	}
}

func toDAGRunDetails(s execution.DAGRunStatus) api.DAGRunDetails {
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
		Preconditions:    ptrOf(preconditions),
		Nodes:            nodes,
		OnSuccess:        ptrOf(toNode(s.OnSuccess)),
		OnFailure:        ptrOf(toNode(s.OnFailure)),
		OnCancel:         ptrOf(toNode(s.OnCancel)),
		OnExit:           ptrOf(toNode(s.OnExit)),
	}
}

func toNode(node *execution.Node) api.Node {
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

func toSubDAGRuns(subDAGRuns []execution.SubDAGRun) []api.SubDAGRun {
	var result []api.SubDAGRun
	for _, w := range subDAGRuns {
		subDAGRun := api.SubDAGRun{
			DagRunId: w.DAGRunID,
		}
		if w.Params != "" {
			subDAGRun.Params = &w.Params
		}
		result = append(result, subDAGRun)
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
	var details *api.DAGDetails
	if dag == nil {
		return details
	}

	var steps []api.Step
	for _, step := range dag.Steps {
		steps = append(steps, toStep(step))
	}

	handlers := dag.HandlerOn

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

	var schedules []api.Schedule
	for _, s := range dag.Schedule {
		schedules = append(schedules, api.Schedule{
			Expression: s.Expression,
		})
	}

	var preconditions []api.Condition
	for _, p := range dag.Preconditions {
		preconditions = append(preconditions, toPrecondition(p))
	}

	var runConfig *api.RunConfig = nil

	if dag.RunConfig != nil {
		runConfig = &api.RunConfig{
			DisableParamEdit: dag.RunConfig.DisableParamEdit,
			DisableRunIdEdit: dag.RunConfig.DisableRunIdEdit,
		}
	}

	ret := &api.DAGDetails{
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
		Tags:              ptrOf(dag.Tags),
		RunConfig:         runConfig,
	}

	return ret
}
