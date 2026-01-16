package api

import (
	"context"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
)

// ListQueues implements api.StrictServerInterface.
func (a *API) ListQueues(ctx context.Context, _ api.ListQueuesRequestObject) (api.ListQueuesResponseObject, error) {
	// Map to track queues and their DAG runs
	queueMap := make(map[string]*queueInfo)

	// Track statistics
	var totalRunning, totalQueued, totalCapacity int

	// 1. First, add all configured queues from config.yaml
	// This ensures configured queues appear even when empty
	if a.config.Queues.Enabled && a.config.Queues.Config != nil {
		for _, queueCfg := range a.config.Queues.Config {
			queue := &queueInfo{
				name:           queueCfg.Name,
				queueType:      "global",
				maxConcurrency: queueCfg.MaxActiveRuns,
				running:        []api.DAGRunSummary{},
				queuedCount:    0,
			}
			queueMap[queueCfg.Name] = queue
		}
	}

	// 2. Get all running DAG runs from ProcStore (real-time with heartbeats)
	runningByGroup, err := a.procStore.ListAllAlive(ctx)
	if err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Failed to list running processes",
			HTTPStatus: 500,
		}
	}

	// Process running DAG runs
	for groupName, dagRuns := range runningByGroup {
		var dag *core.DAG
		var queue *queueInfo

		// Convert each running DAG run to DAGRunSummary
		for _, dagRun := range dagRuns {
			// Get the DAG run attempt
			attempt, err := a.dagRunStore.FindAttempt(ctx, dagRun)
			if err != nil {
				continue // Skip if we can't find the attempt
			}

			// Get the DAG from the attempt (only once for the group)
			if dag == nil {
				dag, _ = attempt.ReadDAG(ctx)
			}

			// Get or create queue with the DAG info (only once for the group)
			if queue == nil {
				queue = getOrCreateQueue(queueMap, groupName, a.config, dag)
			}

			// Get the status and add to queue
			runStatus, err := attempt.ReadStatus(ctx)
			if err != nil {
				continue // Skip if we can't read status
			}

			runSummary := toDAGRunSummary(*runStatus)
			queue.running = append(queue.running, runSummary)
			totalRunning++
		}
	}

	// 3. Get queued COUNTS only (NOT full items) using QueueList + Len
	queueNames, err := a.queueStore.QueueList(ctx)
	if err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Failed to list queue names",
			HTTPStatus: 500,
		}
	}

	for _, queueName := range queueNames {
		count, err := a.queueStore.Len(ctx, queueName)
		if err != nil {
			logger.Warn(ctx, "Failed to get queue length",
				tag.Queue(queueName),
				tag.Error(err))
			continue
		}

		queue := getOrCreateQueue(queueMap, queueName, a.config, nil)
		queue.queuedCount = count
		totalQueued += count
	}

	// Convert map to slice and calculate total capacity
	queues := make([]api.Queue, 0, len(queueMap))
	for _, q := range queueMap {
		queue := api.Queue{
			Name:         q.name,
			Type:         api.QueueType(q.queueType),
			Running:      q.running,
			RunningCount: len(q.running),
			QueuedCount:  q.queuedCount,
		}

		// Include maxConcurrency for both global and DAG-based queues
		if q.maxConcurrency > 0 {
			queue.MaxConcurrency = &q.maxConcurrency
			totalCapacity += q.maxConcurrency
		}

		queues = append(queues, queue)
	}

	// Calculate utilization percentage
	var utilizationPercentage float32
	if totalCapacity > 0 {
		utilizationPercentage = float32(totalRunning) / float32(totalCapacity) * 100
	}

	// Build response
	response := api.QueuesResponse{
		Queues: queues,
		Summary: api.QueuesSummary{
			TotalQueues:           len(queues),
			TotalRunning:          totalRunning,
			TotalQueued:           totalQueued,
			TotalCapacity:         totalCapacity,
			UtilizationPercentage: utilizationPercentage,
		},
	}

	return api.ListQueues200JSONResponse(response), nil
}

// ListQueueItems implements api.StrictServerInterface.
// Returns paginated items for a specific queue.
func (a *API) ListQueueItems(ctx context.Context, req api.ListQueueItemsRequestObject) (api.ListQueueItemsResponseObject, error) {
	queueName := req.Name
	itemType := api.ListQueueItemsParamsTypeQueued
	if req.Params.Type != nil {
		itemType = *req.Params.Type
	}

	// Create Paginator from page/perPage params
	pg := exec.NewPaginator(valueOf(req.Params.Page), valueOf(req.Params.PerPage))

	var items []api.DAGRunSummary

	if itemType == api.ListQueueItemsParamsTypeRunning {
		// Get running items from proc store (bounded by maxConcurrency)
		runningByGroup, err := a.procStore.ListAllAlive(ctx)
		if err != nil {
			return nil, &Error{
				Code:       api.ErrorCodeInternalError,
				Message:    "Failed to list running processes",
				HTTPStatus: 500,
			}
		}

		runningRefs := runningByGroup[queueName]
		total := len(runningRefs)

		// Apply pagination manually for running items
		offset := pg.Offset()
		limit := pg.Limit()
		if offset < total {
			endIndex := offset + limit
			if endIndex > total {
				endIndex = total
			}
			for _, dagRun := range runningRefs[offset:endIndex] {
				attempt, err := a.dagRunStore.FindAttempt(ctx, dagRun)
				if err != nil {
					continue
				}
				runStatus, err := attempt.ReadStatus(ctx)
				if err != nil {
					continue
				}
				items = append(items, toDAGRunSummary(*runStatus))
			}
		}

		// Create paginated result for response
		paginatedResult := exec.NewPaginatedResult(items, total, pg)
		return api.ListQueueItems200JSONResponse{
			Items:      items,
			Pagination: toPagination(paginatedResult),
		}, nil
	}

	// Get queued items with pagination using ListPaginated
	paginatedResult, err := a.queueStore.ListPaginated(ctx, queueName, pg)
	if err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Failed to list queued items",
			HTTPStatus: 500,
		}
	}

	// Convert QueuedItemData to DAGRunSummary
	for _, queuedItem := range paginatedResult.Items {
		dagRunRef, err := queuedItem.Data()
		if err != nil {
			continue
		}
		attempt, err := a.dagRunStore.FindAttempt(ctx, *dagRunRef)
		if err != nil {
			continue
		}
		runStatus, err := attempt.ReadStatus(ctx)
		if err != nil {
			continue
		}
		items = append(items, toDAGRunSummary(*runStatus))
	}

	return api.ListQueueItems200JSONResponse{
		Items:      items,
		Pagination: toPagination(paginatedResult),
	}, nil
}

// Helper struct to build queue information
type queueInfo struct {
	name           string
	queueType      string
	maxConcurrency int
	running        []api.DAGRunSummary
	queuedCount    int
}

// Helper function to get or create queue in the map
func getOrCreateQueue(queueMap map[string]*queueInfo, queueName string, config *config.Config, dag *core.DAG) *queueInfo {
	queue, exists := queueMap[queueName]
	if !exists {
		queue = &queueInfo{
			name:        queueName,
			queueType:   "dag-based", // Default to dag-based
			running:     []api.DAGRunSummary{},
			queuedCount: 0,
		}

		// Check if this is a global queue from config
		if isGlobalQueue(queueName, config) {
			queue.queueType = "global"
			queue.maxConcurrency = getQueueMaxConcurrency(queueName, config)
		} else if dag != nil {
			// For DAG-based queues, use the DAG's MaxActiveRuns
			queue.maxConcurrency = dag.MaxActiveRuns
		}

		queueMap[queueName] = queue
	}
	return queue
}

// Helper function to check if a queue is global (defined in config)
func isGlobalQueue(queueName string, config *config.Config) bool {
	if config.Queues.Enabled && config.Queues.Config != nil {
		for _, queueCfg := range config.Queues.Config {
			if queueCfg.Name == queueName {
				return true
			}
		}
	}
	return false
}

// Helper function to get queue max concurrency from config
func getQueueMaxConcurrency(queueName string, config *config.Config) int {
	if config.Queues.Enabled && config.Queues.Config != nil {
		for _, queueCfg := range config.Queues.Config {
			if queueCfg.Name == queueName {
				return queueCfg.MaxActiveRuns
			}
		}
	}

	// Default to 1 if not found
	return 1
}
