package response

import (
	"github.com/dagu-dev/dagu/internal/constants"
	domain "github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/service/frontend/models"
	"github.com/samber/lo"
	"sort"
	"strings"
)

func ToDagLogResponse(logs []*domain.StatusFile) *models.DagLogResponse {
	statusByName := map[string][]scheduler.NodeStatus{}
	for i, l := range logs {
		for _, node := range l.Status.Nodes {
			addStatusGridItem(statusByName, len(logs), i, node)
		}
	}

	grid := lo.MapToSlice(statusByName, func(k string, v []scheduler.NodeStatus) *models.DagLogGridItem {
		return ToDagLogGridItem(k, v)
	})

	sort.Slice(grid, func(i, c int) bool {
		return strings.Compare(lo.FromPtr(grid[i].Name), lo.FromPtr(grid[c].Name)) <= 0
	})

	hookStatusByName := map[string][]scheduler.NodeStatus{}
	for i, l := range logs {
		if l.Status.OnSuccess != nil {
			addStatusGridItem(hookStatusByName, len(logs), i, l.Status.OnSuccess)
		}
		if l.Status.OnFailure != nil {
			addStatusGridItem(hookStatusByName, len(logs), i, l.Status.OnFailure)
		}
		if l.Status.OnCancel != nil {
			addStatusGridItem(hookStatusByName, len(logs), i, l.Status.OnCancel)
		}
		if l.Status.OnExit != nil {
			addStatusGridItem(hookStatusByName, len(logs), i, l.Status.OnExit)
		}
	}
	for _, k := range []string{constants.OnSuccess, constants.OnFailure, constants.OnCancel, constants.OnExit} {
		if v, ok := hookStatusByName[k]; ok {
			grid = append(grid, ToDagLogGridItem(k, v))
		}
	}

	converted := lo.Map(logs, func(item *domain.StatusFile, _ int) *models.DagStatusFile {
		return ToDagStatusFile(item)
	})

	ret := &models.DagLogResponse{
		Logs:     lo.Reverse(converted),
		GridData: grid,
	}

	return ret
}

func addStatusGridItem(data map[string][]scheduler.NodeStatus, logLen, logIdx int, node *domain.Node) {
	if _, ok := data[node.Name]; !ok {
		data[node.Name] = make([]scheduler.NodeStatus, logLen)
	}
	data[node.Name][logIdx] = node.Status
}

func ToDagStatusFile(status *domain.StatusFile) *models.DagStatusFile {
	return &models.DagStatusFile{
		File:   lo.ToPtr(status.File),
		Status: ToDagStatusDetail(status.Status),
	}
}

func ToDagLogGridItem(name string, vals []scheduler.NodeStatus) *models.DagLogGridItem {
	return &models.DagLogGridItem{
		Name: lo.ToPtr(name),
		Vals: lo.Map(vals, func(item scheduler.NodeStatus, _ int) int64 {
			return int64(item)
		}),
	}
}

func ToDagStepLogResponse(logFile, content string, step *domain.Node) *models.DagStepLogResponse {
	return &models.DagStepLogResponse{
		LogFile: lo.ToPtr(logFile),
		Step:    ToNode(step),
		Content: lo.ToPtr(content),
	}
}

func ToDagSchedulerLogResponse(logFile, content string) *models.DagSchedulerLogResponse {
	return &models.DagSchedulerLogResponse{
		LogFile: lo.ToPtr(logFile),
		Content: lo.ToPtr(content),
	}
}
