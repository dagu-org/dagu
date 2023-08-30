package handlers

import (
	"fmt"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/persistence/jsondb"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/storage"
	"github.com/yohamta/dagu/internal/suspend"
	"net/http"
	"path"
	"path/filepath"
)

func handleGetDAG() http.HandlerFunc {
	renderFunc := useTemplate("index.gohtml", "dag")
	return func(w http.ResponseWriter, r *http.Request) {
		renderFunc(w, nil)
	}
}

func handlePostDAG() http.HandlerFunc {
	cfg := config.Get()

	return func(w http.ResponseWriter, r *http.Request) {
		action := r.FormValue("action")
		value := r.FormValue("value")
		reqId := r.FormValue("request-id")
		step := r.FormValue("step")
		params := r.FormValue("params")

		dn := dagNameFromCtx(r.Context())

		file := filepath.Join(cfg.DAGs, fmt.Sprintf("%s.yaml", dn))
		dr := controller.NewDAGStatusReader(jsondb.New())
		d, err := dr.ReadStatus(file, false)
		if err != nil && action != "save" {
			encodeError(w, err)
			return
		}
		c := controller.New(d.DAG, jsondb.New())

		switch action {
		case "start":
			if d.Status.Status == scheduler.SchedulerStatus_Running {
				encodeError(w, fmt.Errorf("already running: %w", errInvalidArgs))
				return
			}
			c.StartAsync(cfg.Command, cfg.WorkDir, params)

		case "suspend":
			sc := suspend.NewSuspendChecker(storage.NewStorage(config.Get().SuspendFlagsDir))
			_ = sc.ToggleSuspend(d.DAG, value == "true")

		case "stop":
			if d.Status.Status != scheduler.SchedulerStatus_Running {
				encodeError(w, fmt.Errorf("the DAG is not running: %w", errInvalidArgs))
				return
			}
			err = c.Stop()
			if err != nil {
				encodeError(w, fmt.Errorf("error trying to stop the DAG: %w", err))
				return
			}

		case "retry":
			if reqId == "" {
				encodeError(w, fmt.Errorf("request-id is required: %w", errInvalidArgs))
				return
			}
			err = c.Retry(cfg.Command, cfg.WorkDir, reqId)
			if err != nil {
				encodeError(w, err)
				return
			}

		case "mark-success":
			if d.Status.Status == scheduler.SchedulerStatus_Running {
				encodeError(w, fmt.Errorf("the DAG is still running: %w", errInvalidArgs))
				return
			}
			if reqId == "" {
				encodeError(w, fmt.Errorf("request-id is required: %w", errInvalidArgs))
				return
			}
			if step == "" {
				encodeError(w, fmt.Errorf("step name is required: %w", errInvalidArgs))
				return
			}

			err = updateStatus(c, reqId, step, scheduler.NodeStatus_Success)
			if err != nil {
				encodeError(w, err)
				return
			}

		case "mark-failed":
			if d.Status.Status == scheduler.SchedulerStatus_Running {
				encodeError(w, fmt.Errorf("the DAG is still running: %w", errInvalidArgs))
				return
			}
			if reqId == "" {
				encodeError(w, fmt.Errorf("request-id is required: %w", errInvalidArgs))
				return
			}
			if step == "" {
				encodeError(w, fmt.Errorf("step name is required: %w", errInvalidArgs))
				return
			}

			err = updateStatus(c, reqId, step, scheduler.NodeStatus_Error)
			if err != nil {
				encodeError(w, err)
				return
			}

		case "save":
			err := c.UpdateDAGSpec(value)
			if err != nil {
				encodeError(w, err)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
			return

		case "rename":
			newfile := nameWithExt(path.Join(cfg.DAGs, value))
			c := controller.New(d.DAG, jsondb.New())
			err := c.MoveDAG(file, newfile)
			if err != nil {
				encodeError(w, err)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))

		default:
			encodeError(w, errInvalidArgs)
			return
		}

		http.Redirect(w, r, dn, http.StatusSeeOther)
	}
}

func handleDeleteDAG() http.HandlerFunc {
	cfg := config.Get()

	return func(w http.ResponseWriter, r *http.Request) {
		dn := dagNameFromCtx(r.Context())

		file := filepath.Join(cfg.DAGs, fmt.Sprintf("%s.yaml", dn))
		dr := controller.NewDAGStatusReader(jsondb.New())
		d, err := dr.ReadStatus(file, false)
		if err != nil {
			encodeError(w, err)
		}

		c := controller.New(d.DAG, jsondb.New())

		err = c.DeleteDAG()

		if err != nil {
			encodeError(w, err)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}
}

func updateStatus(c *controller.DAGController, reqId, step string, to scheduler.NodeStatus) error {
	status, err := c.GetStatusByRequestId(reqId)
	if err != nil {
		return err
	}
	found := false
	for i := range status.Nodes {
		if status.Nodes[i].Step.Name == step {
			status.Nodes[i].Status = to
			status.Nodes[i].StatusText = to.String()
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("step was not found: %s", step)
	}
	return c.UpdateStatus(status)
}
