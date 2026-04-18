// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	openapi "github.com/dagucloud/dagu/api/v1"
	"github.com/dagucloud/dagu/internal/automata"
	"github.com/dagucloud/dagu/internal/cmn/config"
	"github.com/dagucloud/dagu/internal/persis/filedag"
	"github.com/dagucloud/dagu/internal/persis/filedagrun"
	"github.com/dagucloud/dagu/internal/persis/filememory"
	"github.com/dagucloud/dagu/internal/runtime"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func newAutomataMemoryAPI(t *testing.T) (*API, *automata.Service, *filememory.Store) {
	t.Helper()

	root := t.TempDir()
	dagsDir := filepath.Join(root, "dags")
	dataDir := filepath.Join(root, "data")
	runsDir := filepath.Join(root, "runs")

	require.NoError(t, os.MkdirAll(dagsDir, 0o750))
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	require.NoError(t, os.MkdirAll(runsDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(dagsDir, "build-app.yaml"),
		[]byte("name: build-app\nsteps:\n  - name: step1\n    command: echo hello\n"),
		0o600,
	))

	cfg := &config.Config{
		Server: config.Server{
			Permissions: map[config.Permission]bool{
				config.PermissionWriteDAGs: true,
			},
		},
		Paths: config.PathsConfig{
			DAGsDir:    dagsDir,
			DataDir:    dataDir,
			DAGRunsDir: runsDir,
		},
	}

	dagStore := filedag.New(dagsDir, filedag.WithSkipExamples(true))
	dagRunStore := filedagrun.New(runsDir)
	memoryStore, err := filememory.New(dagsDir)
	require.NoError(t, err)
	automataService := automata.New(
		cfg,
		dagStore,
		dagRunStore,
		automata.WithMemoryStore(memoryStore),
	)

	api := New(
		dagStore,
		dagRunStore,
		nil,
		nil,
		runtime.Manager{},
		cfg,
		nil,
		nil,
		prometheus.NewRegistry(),
		nil,
		WithAutomataService(automataService),
		WithAgentMemoryStore(memoryStore),
	)
	return api, automataService, memoryStore
}

func TestAutomataDocumentEndpoints(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	api, svc, _ := newAutomataMemoryAPI(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", "goal: Complete the assigned software work\nallowed_dags:\n  names:\n    - build-app\n"))

	getResp, err := api.GetAutomataDocument(ctx, openapi.GetAutomataDocumentRequestObject{
		Name:     "software_dev",
		Document: openapi.AutomataDocumentMEMORYMd,
	})
	require.NoError(t, err)
	getOK, ok := getResp.(openapi.GetAutomataDocument200JSONResponse)
	require.True(t, ok)
	require.Equal(t, "software_dev", getOK.Name)
	require.Equal(t, openapi.AutomataDocumentMEMORYMd, getOK.Document)
	require.Empty(t, getOK.Content)
	require.Contains(t, getOK.Path, "/memory/automata/software_dev/MEMORY.md")

	updateResp, err := api.UpdateAutomataDocument(ctx, openapi.UpdateAutomataDocumentRequestObject{
		Name:     "software_dev",
		Document: openapi.AutomataDocumentMEMORYMd,
		Body: &openapi.UpdateAgentMemoryRequest{
			Content: "# Memory\n\nRemember the operating rules.",
		},
	})
	require.NoError(t, err)
	updateOK, ok := updateResp.(openapi.UpdateAutomataDocument200JSONResponse)
	require.True(t, ok)
	require.Contains(t, updateOK.Content, "Remember the operating rules.")

	soulResp, err := api.UpdateAutomataDocument(ctx, openapi.UpdateAutomataDocumentRequestObject{
		Name:     "software_dev",
		Document: openapi.AutomataDocumentSOULMd,
		Body: &openapi.UpdateAgentMemoryRequest{
			Content: "# Soul\n\nBe precise.",
		},
	})
	require.NoError(t, err)
	soulOK, ok := soulResp.(openapi.UpdateAutomataDocument200JSONResponse)
	require.True(t, ok)
	require.Equal(t, openapi.AutomataDocumentSOULMd, soulOK.Document)
	require.Contains(t, soulOK.Path, "/memory/automata/software_dev/SOUL.md")
	require.Contains(t, soulOK.Content, "Be precise.")

	getResp, err = api.GetAutomataDocument(ctx, openapi.GetAutomataDocumentRequestObject{
		Name:     "software_dev",
		Document: openapi.AutomataDocumentMEMORYMd,
	})
	require.NoError(t, err)
	getOK, ok = getResp.(openapi.GetAutomataDocument200JSONResponse)
	require.True(t, ok)
	require.Contains(t, getOK.Content, "Remember the operating rules.")

	deleteResp, err := api.DeleteAutomataDocument(ctx, openapi.DeleteAutomataDocumentRequestObject{
		Name:     "software_dev",
		Document: openapi.AutomataDocumentMEMORYMd,
	})
	require.NoError(t, err)
	_, ok = deleteResp.(openapi.DeleteAutomataDocument204Response)
	require.True(t, ok)

	getResp, err = api.GetAutomataDocument(ctx, openapi.GetAutomataDocumentRequestObject{
		Name:     "software_dev",
		Document: openapi.AutomataDocumentMEMORYMd,
	})
	require.NoError(t, err)
	getOK, ok = getResp.(openapi.GetAutomataDocument200JSONResponse)
	require.True(t, ok)
	require.Empty(t, getOK.Content)

	soulGetResp, err := api.GetAutomataDocument(ctx, openapi.GetAutomataDocumentRequestObject{
		Name:     "software_dev",
		Document: openapi.AutomataDocumentSOULMd,
	})
	require.NoError(t, err)
	soulGetOK, ok := soulGetResp.(openapi.GetAutomataDocument200JSONResponse)
	require.True(t, ok)
	require.Contains(t, soulGetOK.Content, "Be precise.")
}
