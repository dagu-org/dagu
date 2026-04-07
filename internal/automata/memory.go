// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dagucloud/dagu/internal/cmn/fileutil"
)

func (s *Service) GetMemory(ctx context.Context, name string) (*Memory, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	if _, err := s.GetDefinition(ctx, name); err != nil {
		return nil, err
	}
	if s.memoryStore == nil {
		return nil, errors.New("automata memory store is not available")
	}
	content, err := s.memoryStore.LoadAutomataMemory(ctx, name)
	if err != nil {
		return nil, err
	}
	path, err := s.memoryStore.AutomataMemoryPath(name)
	if err != nil {
		return nil, err
	}
	return &Memory{Name: name, Content: content, Path: path}, nil
}

func (s *Service) SaveMemory(ctx context.Context, name, content string) (*Memory, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	if _, err := s.GetDefinition(ctx, name); err != nil {
		return nil, err
	}
	if s.memoryStore == nil {
		return nil, errors.New("automata memory store is not available")
	}
	if err := s.memoryStore.SaveAutomataMemory(ctx, name, content); err != nil {
		return nil, err
	}
	return s.GetMemory(ctx, name)
}

func (s *Service) DeleteMemory(ctx context.Context, name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if _, err := s.GetDefinition(ctx, name); err != nil {
		return err
	}
	if s.memoryStore == nil {
		return errors.New("automata memory store is not available")
	}
	return s.memoryStore.DeleteAutomataMemory(ctx, name)
}

func (s *Service) copyMemoryFile(_ context.Context, srcName, dstName string) error {
	if s.memoryStore == nil {
		return nil
	}
	srcPath, err := s.memoryStore.AutomataMemoryPath(srcName)
	if err != nil {
		return err
	}
	dstPath, err := s.memoryStore.AutomataMemoryPath(dstName)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(srcPath) //nolint:gosec // path validated by memory store
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read automata memory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), dirPerm); err != nil {
		return fmt.Errorf("create automata memory dir: %w", err)
	}
	if err := fileutil.WriteFileAtomic(dstPath, data, definitionFilePerm); err != nil {
		return fmt.Errorf("write automata memory: %w", err)
	}
	return nil
}

func (s *Service) moveMemoryFile(_ context.Context, srcName, dstName string) error {
	if s.memoryStore == nil {
		return nil
	}
	srcPath, err := s.memoryStore.AutomataMemoryPath(srcName)
	if err != nil {
		return err
	}
	dstPath, err := s.memoryStore.AutomataMemoryPath(dstName)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(srcPath) //nolint:gosec // path validated by memory store
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read automata memory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), dirPerm); err != nil {
		return fmt.Errorf("create automata memory dir: %w", err)
	}
	if err := fileutil.WriteFileAtomic(dstPath, data, definitionFilePerm); err != nil {
		return fmt.Errorf("write automata memory: %w", err)
	}
	if err := os.RemoveAll(filepath.Dir(srcPath)); err != nil {
		_ = os.RemoveAll(filepath.Dir(dstPath))
		return fmt.Errorf("remove old automata memory dir: %w", err)
	}
	return nil
}

func (s *Service) removeMemoryFile(ctx context.Context, name string) error {
	if s.memoryStore == nil {
		return nil
	}
	return s.memoryStore.DeleteAutomataMemory(ctx, name)
}
