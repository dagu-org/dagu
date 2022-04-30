package controller

import (
	"fmt"
	"path/filepath"

	"github.com/yohamta/dagman/internal/config"
	"github.com/yohamta/dagman/internal/models"
	"github.com/yohamta/dagman/internal/scheduler"
	"github.com/yohamta/dagman/internal/utils"
)

type DAG struct {
	File   string
	Dir    string
	Config *config.Config
	Status *models.Status
	Error  error
	ErrorT *string
}

func FromConfig(file string) (*DAG, error) {
	if !utils.FileExists(file) {
		return nil, fmt.Errorf("file not found: %s", file)
	}
	return fromConfig(file, false)
}

func fromConfig(file string, headOnly bool) (*DAG, error) {
	cl := config.NewConfigLoader()
	var cfg *config.Config
	var err error
	if headOnly {
		cfg, err = cl.LoadHeadOnly(file)
	} else {
		cfg, err = cl.Load(file, "")
	}
	if err != nil {
		if cfg != nil {
			return newDAG(cfg, nil, err), err
		}
		cfg := &config.Config{ConfigPath: file}
		cfg.Init()
		return newDAG(cfg, nil, err), err
	}
	status, err := New(cfg).GetLastStatus()
	if err != nil {
		return nil, err
	}
	if !headOnly {
		if _, err := scheduler.NewExecutionGraph(cfg.Steps...); err != nil {
			return newDAG(cfg, status, err), err
		}
	}
	return newDAG(cfg, status, err), nil
}

func newDAG(cfg *config.Config, s *models.Status, err error) *DAG {
	ret := &DAG{
		File:   filepath.Base(cfg.ConfigPath),
		Dir:    filepath.Dir(cfg.ConfigPath),
		Config: cfg,
		Status: s,
		Error:  err,
	}
	if err != nil {
		errT := err.Error()
		ret.ErrorT = &errT
	}
	return ret
}
