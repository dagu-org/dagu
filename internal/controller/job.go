package controller

import (
	"jobctl/internal/config"
	"jobctl/internal/models"
	"jobctl/internal/scheduler"
	"path/filepath"
)

type Job struct {
	File   string
	Dir    string
	Config *config.Config
	Status *models.Status
	Error  error
	ErrorT *string
}

func FromConfig(file string) (*Job, error) {
	return fromConfig(file, false)
}

func fromConfig(file string, headOnly bool) (*Job, error) {
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
			return newJob(cfg, nil, err), err
		}
		cfg := &config.Config{ConfigPath: file}
		cfg.Init()
		return newJob(cfg, nil, err), err
	}
	status, err := New(cfg).GetLastStatus()
	if err != nil {
		return nil, err
	}
	if !headOnly {
		if _, err := scheduler.NewExecutionGraph(cfg.Steps...); err != nil {
			return newJob(cfg, status, err), err
		}
	}
	return newJob(cfg, status, err), nil
}

func newJob(cfg *config.Config, s *models.Status, err error) *Job {
	ret := &Job{
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
