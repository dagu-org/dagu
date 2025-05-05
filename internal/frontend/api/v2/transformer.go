package api

import (
	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/runstore"
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
	var conditions []api.Precondition
	for _, cond := range obj.Preconditions {
		conditions = append(conditions, toPrecondition(cond))
	}

	repeatPolicy := api.RepeatPolicy{
		Repeat:   ptrOf(obj.RepeatPolicy.Repeat),
		Interval: ptrOf(int(obj.RepeatPolicy.Interval.Seconds())),
	}

	step := api.Step{
		Name:          obj.Name,
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

	if obj.SubDAG != nil {
		step.Run = ptrOf(obj.SubDAG.Name)
		step.Params = ptrOf(obj.SubDAG.Params)
	}
	return step
}

func toPrecondition(obj digraph.Condition) api.Precondition {
	return api.Precondition{
		Condition: ptrOf(obj.Condition),
		Expected:  ptrOf(obj.Expected),
	}
}

func toRunDetails(s runstore.Status) api.RunDetails {
	status := api.RunDetails{
		Log:         s.Log,
		Name:        s.Name,
		Params:      ptrOf(s.Params),
		Pid:         ptrOf(int(s.PID)),
		RequestId:   s.RequestID,
		StartedAt:   s.StartedAt,
		FinishedAt:  s.FinishedAt,
		Status:      api.Status(s.Status),
		StatusLabel: api.StatusLabel(s.Status.String()),
	}
	for _, n := range s.Nodes {
		status.Nodes = append(status.Nodes, toNode(n))
	}
	if s.OnSuccess != nil {
		status.OnSuccess = ptrOf(toNode(s.OnSuccess))
	}
	if s.OnFailure != nil {
		status.OnFailure = ptrOf(toNode(s.OnFailure))
	}
	if s.OnCancel != nil {
		status.OnCancel = ptrOf(toNode(s.OnCancel))
	}
	if s.OnExit != nil {
		status.OnExit = ptrOf(toNode(s.OnExit))
	}
	return status
}

func toNode(node *runstore.Node) api.Node {
	return api.Node{
		DoneCount:   node.DoneCount,
		FinishedAt:  node.FinishedAt,
		Log:         node.Log,
		RetryCount:  node.RetryCount,
		StartedAt:   node.StartedAt,
		Status:      api.NodeStatus(node.Status),
		StatusLabel: api.NodeStatusLabel(node.Status.String()),
		Step:        toStep(node.Step),
		Error:       ptrOf(node.Error),
		SubRuns:     ptrOf(toSubRuns(node.SubRuns)),
	}
}

func toSubRuns(subRuns []runstore.SubRun) []api.SubRun {
	var result []api.SubRun
	for _, subRun := range subRuns {
		result = append(result, api.SubRun{
			RequestId: subRun.RequestID,
		})
	}
	return result
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

	var preconditions []api.Precondition
	for _, p := range dag.Preconditions {
		preconditions = append(preconditions, toPrecondition(p))
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
		Params:            ptrOf(dag.Params),
		Preconditions:     ptrOf(preconditions),
		Schedule:          ptrOf(schedules),
		Steps:             ptrOf(steps),
		Tags:              ptrOf(dag.Tags),
	}
}
