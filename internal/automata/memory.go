// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

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

var automataDocumentNames = []string{DocumentMemory, DocumentSoul}

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
	content, err := store.LoadAutomataDocument(ctx, name, document)
	if err != nil {
		return nil, err
	}
	path, err := store.AutomataDocumentPath(name, document)
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
	if err := store.SaveAutomataDocument(ctx, name, document, content); err != nil {
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
	return store.DeleteAutomataDocument(ctx, name, document)
}

func (s *Service) documentStore() (agent.AutomataDocumentStore, error) {
	if s.memoryStore == nil {
		return nil, errors.New("automata document store is not available")
	}
	store, ok := s.memoryStore.(agent.AutomataDocumentStore)
	if !ok {
		return nil, errors.New("automata document store is not available")
	}
	return store, nil
}

func validateDocumentName(document string) error {
	switch document {
	case DocumentMemory, DocumentSoul:
		return nil
	default:
		return fmt.Errorf("invalid automata document %q", document)
	}
}

func (s *Service) copyMemoryFile(_ context.Context, srcName, dstName string) error {
	if s.memoryStore == nil {
		return nil
	}
	if store, ok := s.memoryStore.(agent.AutomataDocumentStore); ok {
		return copyAutomataDocuments(store, srcName, dstName)
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
	if store, ok := s.memoryStore.(agent.AutomataDocumentStore); ok {
		return moveAutomataDocuments(store, srcName, dstName)
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
	if store, ok := s.memoryStore.(agent.AutomataDocumentStore); ok {
		path, err := store.AutomataDocumentPath(name, DocumentMemory)
		if err != nil {
			return err
		}
		if err := os.RemoveAll(filepath.Dir(path)); err != nil {
			return fmt.Errorf("remove automata document dir: %w", err)
		}
		return nil
	}
	return s.memoryStore.DeleteAutomataMemory(ctx, name)
}

func copyAutomataDocuments(store agent.AutomataDocumentStore, srcName, dstName string) error {
	for _, document := range automataDocumentNames {
		srcPath, err := store.AutomataDocumentPath(srcName, document)
		if err != nil {
			return err
		}
		dstPath, err := store.AutomataDocumentPath(dstName, document)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(srcPath) //nolint:gosec // path validated by memory store
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("read automata document %s: %w", document, err)
		}
		if err := os.MkdirAll(filepath.Dir(dstPath), dirPerm); err != nil {
			return fmt.Errorf("create automata document dir: %w", err)
		}
		if err := fileutil.WriteFileAtomic(dstPath, data, definitionFilePerm); err != nil {
			return fmt.Errorf("write automata document %s: %w", document, err)
		}
	}
	return nil
}

func moveAutomataDocuments(store agent.AutomataDocumentStore, srcName, dstName string) error {
	if err := copyAutomataDocuments(store, srcName, dstName); err != nil {
		dstPath, pathErr := store.AutomataDocumentPath(dstName, DocumentMemory)
		if pathErr == nil {
			_ = os.RemoveAll(filepath.Dir(dstPath))
		}
		return err
	}
	srcPath, err := store.AutomataDocumentPath(srcName, DocumentMemory)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Dir(srcPath)); err != nil {
		dstPath, pathErr := store.AutomataDocumentPath(dstName, DocumentMemory)
		if pathErr == nil {
			_ = os.RemoveAll(filepath.Dir(dstPath))
		}
		return fmt.Errorf("remove old automata document dir: %w", err)
	}
	return nil
}
