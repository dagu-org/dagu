// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package autopilot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/cmn/fileutil"
)

const (
	DocumentMemory = "MEMORY.md"
	DocumentSoul   = "SOUL.md"
)

var autopilotDocumentNames = []string{DocumentMemory, DocumentSoul}

func (s *Service) GetMemory(ctx context.Context, name string) (*Memory, error) {
	return s.GetDocument(ctx, name, DocumentMemory)
}

func (s *Service) SaveMemory(ctx context.Context, name, content string) (*Memory, error) {
	return s.SaveDocument(ctx, name, DocumentMemory, content)
}

func (s *Service) DeleteMemory(ctx context.Context, name string) error {
	return s.DeleteDocument(ctx, name, DocumentMemory)
}

func (s *Service) GetDocument(ctx context.Context, name, document string) (*Document, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	if err := validateDocumentName(document); err != nil {
		return nil, err
	}
	if _, err := s.GetDefinition(ctx, name); err != nil {
		return nil, err
	}
	store, err := s.documentStore()
	if err != nil {
		return nil, err
	}
	content, err := store.LoadAutopilotDocument(ctx, name, document)
	if err != nil {
		return nil, err
	}
	path, err := store.AutopilotDocumentPath(name, document)
	if err != nil {
		return nil, err
	}
	return &Document{Name: name, Document: document, Content: content, Path: path}, nil
}

func (s *Service) SaveDocument(ctx context.Context, name, document, content string) (*Document, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	if err := validateDocumentName(document); err != nil {
		return nil, err
	}
	if _, err := s.GetDefinition(ctx, name); err != nil {
		return nil, err
	}
	store, err := s.documentStore()
	if err != nil {
		return nil, err
	}
	if err := store.SaveAutopilotDocument(ctx, name, document, content); err != nil {
		return nil, err
	}
	return s.GetDocument(ctx, name, document)
}

func (s *Service) DeleteDocument(ctx context.Context, name, document string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if err := validateDocumentName(document); err != nil {
		return err
	}
	if _, err := s.GetDefinition(ctx, name); err != nil {
		return err
	}
	store, err := s.documentStore()
	if err != nil {
		return err
	}
	return store.DeleteAutopilotDocument(ctx, name, document)
}

func (s *Service) documentStore() (agent.AutopilotDocumentStore, error) {
	if s.memoryStore == nil {
		return nil, errors.New("autopilot document store is not available")
	}
	store, ok := s.memoryStore.(agent.AutopilotDocumentStore)
	if !ok {
		return nil, errors.New("autopilot document store is not available")
	}
	return store, nil
}

func validateDocumentName(document string) error {
	switch document {
	case DocumentMemory, DocumentSoul:
		return nil
	default:
		return fmt.Errorf("invalid autopilot document %q", document)
	}
}

func (s *Service) copyMemoryFile(_ context.Context, srcName, dstName string) error {
	if s.memoryStore == nil {
		return nil
	}
	if store, ok := s.memoryStore.(agent.AutopilotDocumentStore); ok {
		return copyAutopilotDocuments(store, srcName, dstName)
	}
	srcPath, err := s.memoryStore.AutopilotMemoryPath(srcName)
	if err != nil {
		return err
	}
	dstPath, err := s.memoryStore.AutopilotMemoryPath(dstName)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(srcPath) //nolint:gosec // path validated by memory store
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read autopilot memory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), dirPerm); err != nil {
		return fmt.Errorf("create autopilot memory dir: %w", err)
	}
	if err := fileutil.WriteFileAtomic(dstPath, data, definitionFilePerm); err != nil {
		return fmt.Errorf("write autopilot memory: %w", err)
	}
	return nil
}

func (s *Service) moveMemoryFile(_ context.Context, srcName, dstName string) error {
	if s.memoryStore == nil {
		return nil
	}
	if store, ok := s.memoryStore.(agent.AutopilotDocumentStore); ok {
		return moveAutopilotDocuments(store, srcName, dstName)
	}
	srcPath, err := s.memoryStore.AutopilotMemoryPath(srcName)
	if err != nil {
		return err
	}
	dstPath, err := s.memoryStore.AutopilotMemoryPath(dstName)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(srcPath) //nolint:gosec // path validated by memory store
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read autopilot memory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), dirPerm); err != nil {
		return fmt.Errorf("create autopilot memory dir: %w", err)
	}
	if err := fileutil.WriteFileAtomic(dstPath, data, definitionFilePerm); err != nil {
		return fmt.Errorf("write autopilot memory: %w", err)
	}
	if err := os.RemoveAll(filepath.Dir(srcPath)); err != nil {
		_ = os.RemoveAll(filepath.Dir(dstPath))
		return fmt.Errorf("remove old autopilot memory dir: %w", err)
	}
	return nil
}

func (s *Service) removeMemoryFile(ctx context.Context, name string) error {
	if s.memoryStore == nil {
		return nil
	}
	if store, ok := s.memoryStore.(agent.AutopilotDocumentStore); ok {
		path, err := store.AutopilotDocumentPath(name, DocumentMemory)
		if err != nil {
			return err
		}
		if err := os.RemoveAll(filepath.Dir(path)); err != nil {
			return fmt.Errorf("remove autopilot document dir: %w", err)
		}
		return nil
	}
	return s.memoryStore.DeleteAutopilotMemory(ctx, name)
}

func copyAutopilotDocuments(store agent.AutopilotDocumentStore, srcName, dstName string) error {
	for _, document := range autopilotDocumentNames {
		srcPath, err := store.AutopilotDocumentPath(srcName, document)
		if err != nil {
			return err
		}
		dstPath, err := store.AutopilotDocumentPath(dstName, document)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(srcPath) //nolint:gosec // path validated by memory store
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("read autopilot document %s: %w", document, err)
		}
		if err := os.MkdirAll(filepath.Dir(dstPath), dirPerm); err != nil {
			return fmt.Errorf("create autopilot document dir: %w", err)
		}
		if err := fileutil.WriteFileAtomic(dstPath, data, definitionFilePerm); err != nil {
			return fmt.Errorf("write autopilot document %s: %w", document, err)
		}
	}
	return nil
}

func moveAutopilotDocuments(store agent.AutopilotDocumentStore, srcName, dstName string) error {
	if err := copyAutopilotDocuments(store, srcName, dstName); err != nil {
		dstPath, pathErr := store.AutopilotDocumentPath(dstName, DocumentMemory)
		if pathErr == nil {
			_ = os.RemoveAll(filepath.Dir(dstPath))
		}
		return err
	}
	srcPath, err := store.AutopilotDocumentPath(srcName, DocumentMemory)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Dir(srcPath)); err != nil {
		dstPath, pathErr := store.AutopilotDocumentPath(dstName, DocumentMemory)
		if pathErr == nil {
			_ = os.RemoveAll(filepath.Dir(dstPath))
		}
		return fmt.Errorf("remove old autopilot document dir: %w", err)
	}
	return nil
}
