package api

import (
	"context"
	"errors"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/persistence"
)

// CreateDAG implements api.StrictServerInterface.
func (a *API) CreateDAG(ctx context.Context, request api.CreateDAGRequestObject) (api.CreateDAGResponseObject, error) {
	name, err := a.client.CreateDAG(ctx, request.Body.Name)
	if err != nil {
		if errors.Is(err, persistence.ErrDAGAlreadyExists) {
			return nil, newBadRequestError(api.ErrorCodeBadRequest, err)
		}
		return nil, newInternalError(err)
	}
	return &api.CreateDAG201JSONResponse{
		Name: name,
	}, nil
}

// DeleteDAG implements api.StrictServerInterface.
func (a *API) DeleteDAG(ctx context.Context, request api.DeleteDAGRequestObject) (api.DeleteDAGResponseObject, error) {
	_, err := a.client.GetStatus(ctx, request.Name)
	if err != nil {
		return nil, newNotFoundError(api.ErrorCodeNotFound, err)
	}
	if err := a.client.DeleteDAG(ctx, request.Name); err != nil {
		return nil, newInternalError(err)
	}
	return &api.DeleteDAG204Response{}, nil
}

// GetDAGDetails implements api.StrictServerInterface.
func (a *API) GetDAGDetails(ctx context.Context, request api.GetDAGDetailsRequestObject) (api.GetDAGDetailsResponseObject, error) {
	name := request.Name

	tab := api.DAGDetailTabStatus
	if request.Params.Tab != nil {
		tab = *request.Params.Tab
	}

	status, err := a.client.GetStatus(ctx, name)
	if err != nil {
		return nil, newNotFoundError(api.ErrorCodeNotFound, err)
	}

	var steps []api.Step
	for _, step := range status.DAG.Steps {
		steps = append(steps, toStep(step))
	}

	handlers := status.DAG.HandlerOn

	handlerOn := api.HandlerOn{}
	if handlers.Failure != nil {
		handlerOn.Failure = ptr(toStep(*handlers.Failure))
	}
	if handlers.Success != nil {
		handlerOn.Success = ptr(toStep(*handlers.Success))
	}
	if handlers.Cancel != nil {
		handlerOn.Cancel = ptr(toStep(*handlers.Cancel))
	}
	if handlers.Exit != nil {
		handlerOn.Exit = ptr(toStep(*handlers.Exit))
	}

	var schedules []api.Schedule
	for _, s := range status.DAG.Schedule {
		schedules = append(schedules, api.Schedule{
			Expression: s.Expression,
		})
	}

	var preconditions []api.Precondition
	for _, p := range status.DAG.Preconditions {
		preconditions = append(preconditions, toPrecondition(p))
	}

	dag := status.DAG
	details := api.DAGDetails{
		DefaultParams:     dag.DefaultParams,
		Delay:             int(dag.Delay.Seconds()),
		Description:       dag.Description,
		Env:               dag.Env,
		Group:             dag.Group,
		HandlerOn:         handlerOn,
		HistRetentionDays: dag.HistRetentionDays,
		Location:          dag.Location,
		LogDir:            dag.LogDir,
		MaxActiveRuns:     dag.MaxActiveRuns,
		Name:              dag.Name,
		Params:            dag.Params,
		Preconditions:     preconditions,
		Schedule:          schedules,
		Steps:             steps,
		Tags:              dag.Tags,
	}

	statusDetails := api.DAGStatusFileDetails{
		DAG:       details,
		Error:     value(status.ErrorT),
		File:      status.File,
		Status:    toStatus(status.Status),
		Suspended: status.Suspended,
	}

	if status.Error != nil {
		statusDetails.Error = status.Error.Error()
	}

	resp := &api.GetDAGDetails200JSONResponse{
		Title: status.DAG.Name,
		DAG:   statusDetails,
		Tab:   tab,
	}

	if err != nil {
		resp.Errors = append(resp.Errors, err.Error())
	}

	if err := status.DAG.Validate(); err != nil {
		resp.Errors = append(resp.Errors, err.Error())
	}

	switch tab {
	case api.DAGDetailTabStatus:
		return resp, nil

	case api.DAGDetailTabSpec:
		return a.processSpecRequest(ctx, name, resp)

	case api.DAGDetailTabHistory:
		return a.processLogRequest(ctx, resp, dag)

	case api.DAGDetailTabLog:
		return a.processStepLogRequest(ctx, dag, name, resp)

	case api.DAGDetailTabSchedulerLog:
		return a.processSchedulerLogRequest(ctx, dag, name, resp)

	default:
		panic("unreachable")
	}
}

// ListDAGs implements api.StrictServerInterface.
func (a *API) ListDAGs(ctx context.Context, request api.ListDAGsRequestObject) (api.ListDAGsResponseObject, error) {
	var opts []client.ListStatusOption
	if request.Params.Limit != nil {
		opts = append(opts, client.WithLimit(*request.Params.Limit))
	}
	if request.Params.Page != nil {
		opts = append(opts, client.WithPage(*request.Params.Page))
	}
	if request.Params.Name != nil {
		opts = append(opts, client.WithName(*request.Params.Name))
	}
	if request.Params.Tag != nil {
		opts = append(opts, client.WithTag(*request.Params.Tag))
	}

	result, err := a.client.ListStatus(ctx, opts...)
	if err != nil {
		return nil, newInternalError(err)
	}

	hasErr := len(result.Errors) > 0
	for _, item := range result.Items {
		if item.Error != nil {
			hasErr = true
			break
		}
	}

	resp := &api.ListDAGs200JSONResponse{
		Errors:    ptr(result.Errors),
		PageCount: result.TotalPage,
		HasError:  hasErr,
	}

	for _, item := range result.Items {
		status := api.DAGStatus{
			Log:        ptr(item.Status.Log),
			Name:       item.Status.Name,
			Params:     ptr(item.Status.Params),
			Pid:        ptr(int(item.Status.PID)),
			RequestId:  item.Status.RequestID,
			StartedAt:  item.Status.StartedAt,
			FinishedAt: item.Status.FinishedAt,
			Status:     api.RunStatus(item.Status.Status),
			StatusText: api.RunStatusText(item.Status.StatusText),
		}

		dag := api.DAGStatusFile{
			Error:     item.ErrorT,
			File:      item.File,
			Status:    status,
			Suspended: item.Suspended,
			DAG:       toDAG(item.DAG),
		}

		if item.Error != nil {
			dag.Error = ptr(item.Error.Error())
		}

		resp.DAGs = append(resp.DAGs, dag)
	}

	return resp, nil
}

// ListTags implements api.StrictServerInterface.
func (a *API) ListTags(ctx context.Context, _ api.ListTagsRequestObject) (api.ListTagsResponseObject, error) {
	tags, errs, err := a.client.GetTagList(ctx)
	if err != nil {
		return nil, newInternalError(err)
	}
	return &api.ListTags200JSONResponse{
		Tags:   tags,
		Errors: errs,
	}, nil
}

// PostDAGAction implements api.StrictServerInterface.
func (a *API) PostDAGAction(ctx context.Context, request api.PostDAGActionRequestObject) (api.PostDAGActionResponseObject, error) {
	panic("unimplemented")
}

// SearchDAGs implements api.StrictServerInterface.
func (a *API) SearchDAGs(ctx context.Context, request api.SearchDAGsRequestObject) (api.SearchDAGsResponseObject, error) {
	panic("unimplemented")
}

func toDAG(dag *digraph.DAG) api.DAG {
	var schedules []api.Schedule
	for _, s := range dag.Schedule {
		schedules = append(schedules, api.Schedule{Expression: s.Expression})
	}

	return api.DAG{
		Name:          dag.Name,
		Group:         ptr(dag.Group),
		Description:   dag.Description,
		Params:        ptr(dag.Params),
		DefaultParams: ptr(dag.DefaultParams),
		Tags:          ptr(dag.Tags),
		Schedule:      ptr(schedules),
	}
}

func toStep(obj digraph.Step) api.Step {
	var conditions []api.Precondition
	for _, cond := range obj.Preconditions {
		conditions = append(conditions, toPrecondition(cond))
	}

	repeatPolicy := api.RepeatPolicy{
		Repeat:   ptr(obj.RepeatPolicy.Repeat),
		Interval: ptr(int(obj.RepeatPolicy.Interval.Seconds())),
	}

	step := api.Step{
		Args:          obj.Args,
		CmdWithArgs:   obj.CmdWithArgs,
		Command:       obj.Command,
		Depends:       obj.Depends,
		Description:   obj.Description,
		Dir:           obj.Dir,
		MailOnError:   obj.MailOnError,
		Name:          obj.Name,
		Output:        obj.Output,
		Preconditions: conditions,
		RepeatPolicy:  repeatPolicy,
		Script:        obj.Script,
	}

	if obj.SubWorkflow != nil {
		step.Run = ptr(obj.SubWorkflow.Name)
		step.Params = ptr(obj.SubWorkflow.Params)
	}
	return step
}

func toPrecondition(obj digraph.Condition) api.Precondition {
	return api.Precondition{
		Condition: ptr(obj.Condition),
		Expected:  ptr(obj.Expected),
	}
}

func toStatus(s persistence.Status) api.DAGStatusDetails {
	status := api.DAGStatusDetails{
		Log:        s.Log,
		Name:       s.Name,
		Params:     s.Params,
		Pid:        int(s.PID),
		RequestId:  s.RequestID,
		StartedAt:  s.StartedAt,
		FinishedAt: s.FinishedAt,
		Status:     api.RunStatus(s.Status),
		StatusText: api.RunStatusText(s.StatusText),
	}
	for _, n := range s.Nodes {
		status.Nodes = append(status.Nodes, toNode(n))
	}
	if s.OnSuccess != nil {
		status.OnSuccess = toNode(s.OnSuccess)
	}
	if s.OnFailure != nil {
		status.OnFailure = toNode(s.OnFailure)
	}
	if s.OnCancel != nil {
		status.OnCancel = toNode(s.OnCancel)
	}
	if s.OnExit != nil {
		status.OnExit = toNode(s.OnExit)
	}
	return status
}

func toNode(node *persistence.Node) api.Node {
	return api.Node{
		DoneCount:  node.DoneCount,
		Error:      node.Error,
		FinishedAt: node.FinishedAt,
		Log:        node.Log,
		RetryCount: node.RetryCount,
		StartedAt:  node.StartedAt,
		Status:     api.NodeStatus(node.Status),
		StatusText: api.NodeStatusText(node.StatusText),
		Step:       toStep(node.Step),
	}
}
