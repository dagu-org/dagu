// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

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

var controllerDocumentNames = []string{DocumentMemory, DocumentSoul}

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
	content, err := store.LoadControllerDocument(ctx, name, document)
	if err != nil {
		return nil, err
	}
	path, err := store.ControllerDocumentPath(name, document)
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
	if err := store.SaveControllerDocument(ctx, name, document, content); err != nil {
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
	return store.DeleteControllerDocument(ctx, name, document)
}

func (s *Service) documentStore() (agent.ControllerDocumentStore, error) {
	if s.memoryStore == nil {
		return nil, errors.New("controller document store is not available")
	}
	store, ok := s.memoryStore.(agent.ControllerDocumentStore)
	if !ok {
		return nil, errors.New("controller document store is not available")
	}
	return store, nil
}

func validateDocumentName(document string) error {
	switch document {
	case DocumentMemory, DocumentSoul:
		return nil
	default:
		return fmt.Errorf("invalid controller document %q", document)
	}
}

func (s *Service) copyMemoryFile(_ context.Context, srcName, dstName string) error {
	if s.memoryStore == nil {
		return nil
	}
	if store, ok := s.memoryStore.(agent.ControllerDocumentStore); ok {
		return copyControllerDocuments(store, srcName, dstName)
	}
	srcPath, err := s.memoryStore.ControllerMemoryPath(srcName)
	if err != nil {
		return err
	}
	dstPath, err := s.memoryStore.ControllerMemoryPath(dstName)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(srcPath) //nolint:gosec // path validated by memory store
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read controller memory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), dirPerm); err != nil {
		return fmt.Errorf("create controller memory dir: %w", err)
	}
	if err := fileutil.WriteFileAtomic(dstPath, data, definitionFilePerm); err != nil {
		return fmt.Errorf("write controller memory: %w", err)
	}
	return nil
}

func (s *Service) moveMemoryFile(_ context.Context, srcName, dstName string) error {
	if s.memoryStore == nil {
		return nil
	}
	if store, ok := s.memoryStore.(agent.ControllerDocumentStore); ok {
		return moveControllerDocuments(store, srcName, dstName)
	}
	srcPath, err := s.memoryStore.ControllerMemoryPath(srcName)
	if err != nil {
		return err
	}
	dstPath, err := s.memoryStore.ControllerMemoryPath(dstName)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(srcPath) //nolint:gosec // path validated by memory store
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read controller memory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), dirPerm); err != nil {
		return fmt.Errorf("create controller memory dir: %w", err)
	}
	if err := fileutil.WriteFileAtomic(dstPath, data, definitionFilePerm); err != nil {
		return fmt.Errorf("write controller memory: %w", err)
	}
	if err := os.RemoveAll(filepath.Dir(srcPath)); err != nil {
		_ = os.RemoveAll(filepath.Dir(dstPath))
		return fmt.Errorf("remove old controller memory dir: %w", err)
	}
	return nil
}

func (s *Service) removeMemoryFile(ctx context.Context, name string) error {
	if s.memoryStore == nil {
		return nil
	}
	if store, ok := s.memoryStore.(agent.ControllerDocumentStore); ok {
		path, err := store.ControllerDocumentPath(name, DocumentMemory)
		if err != nil {
			return err
		}
		if err := os.RemoveAll(filepath.Dir(path)); err != nil {
			return fmt.Errorf("remove controller document dir: %w", err)
		}
		return nil
	}
	return s.memoryStore.DeleteControllerMemory(ctx, name)
}

func copyControllerDocuments(store agent.ControllerDocumentStore, srcName, dstName string) error {
	for _, document := range controllerDocumentNames {
		srcPath, err := store.ControllerDocumentPath(srcName, document)
		if err != nil {
			return err
		}
		dstPath, err := store.ControllerDocumentPath(dstName, document)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(srcPath) //nolint:gosec // path validated by memory store
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("read controller document %s: %w", document, err)
		}
		if err := os.MkdirAll(filepath.Dir(dstPath), dirPerm); err != nil {
			return fmt.Errorf("create controller document dir: %w", err)
		}
		if err := fileutil.WriteFileAtomic(dstPath, data, definitionFilePerm); err != nil {
			return fmt.Errorf("write controller document %s: %w", document, err)
		}
	}
	return nil
}

func moveControllerDocuments(store agent.ControllerDocumentStore, srcName, dstName string) error {
	if err := copyControllerDocuments(store, srcName, dstName); err != nil {
		dstPath, pathErr := store.ControllerDocumentPath(dstName, DocumentMemory)
		if pathErr == nil {
			_ = os.RemoveAll(filepath.Dir(dstPath))
		}
		return err
	}
	srcPath, err := store.ControllerDocumentPath(srcName, DocumentMemory)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Dir(srcPath)); err != nil {
		dstPath, pathErr := store.ControllerDocumentPath(dstName, DocumentMemory)
		if pathErr == nil {
			_ = os.RemoveAll(filepath.Dir(dstPath))
		}
		return fmt.Errorf("remove old controller document dir: %w", err)
	}
	return nil
}
