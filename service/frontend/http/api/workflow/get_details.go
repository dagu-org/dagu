package workflow

import (
	"fmt"
	"github.com/samber/lo"
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/constants"
	"github.com/dagu-dev/dagu/internal/controller"
	domain "github.com/dagu-dev/dagu/internal/models"
	"github.com/dagu-dev/dagu/internal/persistence/jsondb"
	"github.com/dagu-dev/dagu/service/frontend/http/api/response"
	"github.com/dagu-dev/dagu/service/frontend/models"
	"github.com/dagu-dev/dagu/service/frontend/restapi/operations"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// TODO: separate API
	dagTabTypeStatus       = "status"
	dagTabTypeSpec         = "spec"
	dagTabTypeHistory      = "history"
	dagTabTypeStepLog      = "log"
	dagTabTypeSchedulerLog = "scheduler-log"
)

func GetDetail(params operations.GetWorkflowDetailParams) (*models.GetWorkflowDetailResponse, *response.CodedError) {
	workflowID := params.WorkflowID

	// TODO: separate API
	// optional params
	tab := dagTabTypeStatus
	if params.Tab != nil {
		tab = *params.Tab
	}

	logFile := params.File
	stepName := params.Step

	// TODO: change this to dependency injection
	cfg := config.Get()

	file := filepath.Join(cfg.DAGs, fmt.Sprintf("%s.yaml", workflowID))
	dr := controller.NewDAGStatusReader(jsondb.New())
	workflowStatus, err := dr.ReadStatus(file, false)
	if workflowStatus == nil {
		return nil, response.NewNotFoundError(err)
	}

	ctrl := controller.New(workflowStatus.DAG, jsondb.New())
	resp := response.ToGetWorkflowDetailResponse(
		workflowStatus,
		tab,
	)

	if err != nil {
		resp.Errors = append(resp.Errors, err.Error())
	}

	switch tab {
	case dagTabTypeStatus:
	case dagTabTypeSpec:
		dagContent, err := readFileContent(file, nil)
		if err != nil {
			return nil, response.NewNotFoundError(err)
		}
		resp.Definition = lo.ToPtr(string(dagContent))

	case dagTabTypeHistory:
		logs := controller.New(workflowStatus.DAG, jsondb.New()).GetRecentStatuses(30)
		resp.LogData = response.ToWorkflowLogResponse(logs)

	case dagTabTypeStepLog:
		stepLog, err := getStepLog(ctrl, lo.FromPtr(logFile), lo.FromPtr(stepName))
		if err != nil {
			return nil, response.NewNotFoundError(err)
		}
		resp.StepLog = stepLog

	case dagTabTypeSchedulerLog:
		schedulerLog, err := readSchedulerLog(ctrl, lo.FromPtr(logFile))
		if err != nil {
			return nil, response.NewNotFoundError(err)
		}
		resp.ScLog = schedulerLog

	default:
	}

	return resp, nil
}

func getStepLog(c *controller.DAGController, logFile, stepName string) (*models.WorkflowStepLogResponse, error) {
	var stepByName = map[string]*domain.Node{
		constants.OnSuccess: nil,
		constants.OnFailure: nil,
		constants.OnCancel:  nil,
		constants.OnExit:    nil,
	}

	var status *domain.Status
	if logFile == "" {
		s, err := c.GetLastStatus()
		if err != nil {
			return nil, fmt.Errorf("failed to read status")
		}
		status = s
	} else {
		s, err := jsondb.ParseFile(logFile)
		if err != nil {
			return nil, fmt.Errorf("error parsing %s: %w", logFile, err)
		}
		status = s
	}

	stepByName[constants.OnSuccess] = status.OnSuccess
	stepByName[constants.OnFailure] = status.OnFailure
	stepByName[constants.OnCancel] = status.OnCancel
	stepByName[constants.OnExit] = status.OnExit

	node, ok := lo.Find(status.Nodes, func(item *domain.Node) bool {
		return item.Name == stepName
	})
	if !ok {
		return nil, fmt.Errorf("step name was not found %s", stepName)
	}

	logContent, err := getLogFileContent(node.Log)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", node.Log, err)
	}

	return response.ToWorkflowStepLogResponse(node.Log, logContent, node), nil
}

func getLogFileContent(fileName string) (string, error) {
	// TODO: fix this to change to dependency injection
	enc := config.Get().LogEncodingCharset

	var decoder *encoding.Decoder
	if strings.ToLower(enc) == "euc-jp" {
		decoder = japanese.EUCJP.NewDecoder()
	}
	logContent, err := readFileContent(fileName, decoder)
	return string(logContent), err
}

// TODO: refactor this
func readFileContent(f string, decoder *encoding.Decoder) ([]byte, error) {
	if decoder == nil {
		return os.ReadFile(f)
	}

	r, err := os.Open(f)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", f, err)
	}
	defer func() {
		_ = r.Close()
	}()
	tr := transform.NewReader(r, decoder)
	ret, err := io.ReadAll(tr)
	return ret, err
}

func readSchedulerLog(ctrl *controller.DAGController, statusFile string) (*models.WorkflowSchedulerLogResponse, error) {
	var (
		logFile string
	)
	if statusFile == "" {
		s, err := ctrl.GetLastStatus()
		if err != nil {
			return nil, fmt.Errorf("error reading the last status")
		}
		logFile = s.Log
	} else {
		s, err := jsondb.ParseFile(statusFile)
		if err != nil {
			return nil, fmt.Errorf("error parsing %s: %w", statusFile, err)
		}
		logFile = s.Log
	}
	content, err := readFileContent(logFile, nil)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", logFile, err)
	}
	return response.ToWorkflowSchedulerLogResponse(logFile, string(content)), nil
}
