package response

import (
	"sort"
	"strings"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/internal/service/frontend/gen/models"
	"github.com/go-openapi/swag"
	"github.com/samber/lo"
)

func NewDagLogResponse(logs []*model.StatusFile) *models.DagLogResponse {
	statusByName := map[string][]scheduler.NodeStatus{}
	for i, l := range logs {
		for _, node := range l.Status.Nodes {
			addStatusGridItem(statusByName, len(logs), i, node)
		}
	}

	grid := lo.MapToSlice(
		statusByName,
		func(k string, v []scheduler.NodeStatus) *models.DagLogGridItem {
			return NewDagLogGridItem(k, v)
		},
	)

	sort.Slice(grid, func(i, c int) bool {
		return strings.Compare(lo.FromPtr(grid[i].Name), lo.FromPtr(grid[c].Name)) <= 0
	})

	handlerStatusByName := map[string][]scheduler.NodeStatus{}
	for i, l := range logs {
		if l.Status.OnSuccess != nil {
			addStatusGridItem(handlerStatusByName, len(logs), i, l.Status.OnSuccess)
		}
		if l.Status.OnFailure != nil {
			addStatusGridItem(handlerStatusByName, len(logs), i, l.Status.OnFailure)
		}
		if l.Status.OnCancel != nil {
			addStatusGridItem(handlerStatusByName, len(logs), i, l.Status.OnCancel)
		}
		if l.Status.OnExit != nil {
			addStatusGridItem(handlerStatusByName, len(logs), i, l.Status.OnExit)
		}
	}

	for _, handlerType := range []dag.HandlerType{dag.HandlerOnSuccess, dag.HandlerOnFailure, dag.HandlerOnCancel, dag.HandlerOnExit} {
		if v, ok := handlerStatusByName[handlerType.String()]; ok {
			grid = append(grid, NewDagLogGridItem(handlerType.String(), v))
		}
	}

	converted := lo.Map(logs, func(item *model.StatusFile, _ int) *models.DagStatusFile {
		return NewDagStatusFile(item)
	})

	return &models.DagLogResponse{
		Logs:     lo.Reverse(converted),
		GridData: grid,
	}
}

func addStatusGridItem(
	data map[string][]scheduler.NodeStatus,
	logLen, logIdx int,
	node *model.Node,
) {
	if _, ok := data[node.Name]; !ok {
		data[node.Name] = make([]scheduler.NodeStatus, logLen)
	}
	data[node.Name][logIdx] = node.Status
}

func NewDagStatusFile(status *model.StatusFile) *models.DagStatusFile {
	return &models.DagStatusFile{
		File:   swag.String(status.File),
		Status: NewDagStatusDetail(status.Status),
	}
}

func NewDagLogGridItem(name string, vals []scheduler.NodeStatus) *models.DagLogGridItem {
	return &models.DagLogGridItem{
		Name: swag.String(name),
		Vals: lo.Map(vals, func(item scheduler.NodeStatus, _ int) int64 {
			return int64(item)
		}),
	}
}

func NewDagStepLogResponse(logFile, content string, step *model.Node) *models.DagStepLogResponse {
	return &models.DagStepLogResponse{
		LogFile: swag.String(logFile),
		Step:    NewNode(step),
		Content: swag.String(content),
	}
}

func NewDagSchedulerLogResponse(logFile, content string) *models.DagSchedulerLogResponse {
	return &models.DagSchedulerLogResponse{
		LogFile: swag.String(logFile),
		Content: swag.String(content),
	}
}
