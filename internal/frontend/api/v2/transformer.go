package api

import (
	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/models"
)

func toDAG(dag *digraph.DAG) api.DAG {
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

func toStep(obj digraph.Step) api.Step {
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

	step := api.Step{
		Name:          obj.Name,
		Id:            ptrOf(obj.ID),
		Description:   ptrOf(obj.Description),
		Args:          ptrOf(obj.Args),
		CmdWithArgs:   ptrOf(obj.CmdWithArgs),
		Command:       ptrOf(obj.Command),
		Depends:       ptrOf(obj.Depends),
		Dir:           ptrOf(obj.Dir),
		MailOnError:   ptrOf(obj.MailOnError),
		Output:        ptrOf(obj.Output),
		Preconditions: ptrOf(conditions),
		RepeatPolicy:  ptrOf(repeatPolicy),
		Script:        ptrOf(obj.Script),
	}

	if obj.ChildDAG != nil {
		step.Run = ptrOf(obj.ChildDAG.Name)
		step.Params = ptrOf(obj.ChildDAG.Params)
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
	return step
}

func toPrecondition(obj *digraph.Condition) api.Condition {
	return api.Condition{
		Condition: obj.Condition,
		Expected:  ptrOf(obj.Expected),
		Error:     ptrOf(obj.GetErrorMessage()),
	}
}

func toDAGRunSummary(s models.DAGRunStatus) api.DAGRunSummary {
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
	}
}

func toDAGRunDetails(s models.DAGRunStatus) api.DAGRunDetails {
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
		Preconditions:    ptrOf(preconditions),
		Nodes:            nodes,
		OnSuccess:        ptrOf(toNode(s.OnSuccess)),
		OnFailure:        ptrOf(toNode(s.OnFailure)),
		OnCancel:         ptrOf(toNode(s.OnCancel)),
		OnExit:           ptrOf(toNode(s.OnExit)),
	}
}

func toNode(node *models.Node) api.Node {
	if node == nil {
		return api.Node{}
	}
	return api.Node{
		DoneCount:        node.DoneCount,
		FinishedAt:       node.FinishedAt,
		Stdout:           node.Stdout,
		Stderr:           node.Stderr,
		RetryCount:       node.RetryCount,
		StartedAt:        node.StartedAt,
		Status:           api.NodeStatus(node.Status),
		StatusLabel:      api.NodeStatusLabel(node.Status.String()),
		Step:             toStep(node.Step),
		Error:            ptrOf(node.Error),
		Children:         ptrOf(toChildDAGRuns(node.Children)),
		ChildrenRepeated: ptrOf(toChildDAGRuns(node.ChildrenRepeated)),
	}
}

func toChildDAGRuns(childDAGRuns []models.ChildDAGRun) []api.ChildDAGRun {
	var result []api.ChildDAGRun
	for _, w := range childDAGRuns {
		result = append(result, api.ChildDAGRun{
			DagRunId: w.DAGRunID,
			Params:   w.Params,
		})
	}
	return result
}

func toLocalDAG(dag *digraph.DAG) api.LocalDag {
	return api.LocalDag{
		Name:   dag.Name,
		Dag:    toDAGDetails(dag),
		Errors: []string{},
	}
}

func toDAGDetails(dag *digraph.DAG) *api.DAGDetails {
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
