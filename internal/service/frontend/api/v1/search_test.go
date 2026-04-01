// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api_test

import (
	"context"
	"testing"

	apigen "github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persis/filedag"
	"github.com/dagu-org/dagu/internal/persis/filedoc"
	"github.com/dagu-org/dagu/internal/runtime"
	apiV1 "github.com/dagu-org/dagu/internal/service/frontend/api/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type searchTestSetup struct {
	api      *apiV1.API
	dagStore exec.DAGStore
	docStore agent.DocStore
}

func newSearchTestSetup(t *testing.T, withDocs bool) *searchTestSetup {
	t.Helper()

	dagStore := filedag.New(t.TempDir(), filedag.WithSkipExamples(true))
	cfg := &config.Config{}

	options := []apiV1.APIOption{}
	var docStore agent.DocStore
	if withDocs {
		docStore = filedoc.New(t.TempDir())
		options = append(options, apiV1.WithDocStore(docStore))
	}

	return &searchTestSetup{
		api: apiV1.New(
			dagStore,
			nil,
			nil,
			nil,
			runtime.Manager{},
			cfg,
			nil,
			nil,
			prometheus.NewRegistry(),
			nil,
			options...,
		),
		dagStore: dagStore,
		docStore: docStore,
	}
}

func mustCreateDAG(t *testing.T, setup *searchTestSetup, name, spec string) {
	t.Helper()
	err := setup.dagStore.Create(context.Background(), name, []byte(spec))
	require.NoError(t, err)
}

func mustCreateDoc(t *testing.T, setup *searchTestSetup, id, content string) {
	t.Helper()
	require.NotNil(t, setup.docStore)
	err := setup.docStore.Create(context.Background(), id, content)
	require.NoError(t, err)
}

func TestSearchAll(t *testing.T) {
	t.Parallel()

	t.Run("returns paginated lightweight results with snippet previews", func(t *testing.T) {
		t.Parallel()

		setup := newSearchTestSetup(t, true)

		mustCreateDAG(t, setup, "a-match", `name: a-match
steps:
  - command: echo "Needle."
  - command: echo "needle."
  - command: echo "needle."
  - command: echo "needle."
  - command: echo "needle."`)
		mustCreateDAG(t, setup, "b-match", `name: b-match
steps:
  - command: echo "needle."`)
		mustCreateDAG(t, setup, "c-skip", `name: c-skip
steps:
  - command: echo "needleX"`)

		mustCreateDoc(t, setup, "alpha", "Needle.\nneedle.\nneedle.\nneedle.")
		mustCreateDoc(t, setup, "beta", "needle.")
		mustCreateDoc(t, setup, "gamma", "needleX")

		page := 2
		docPage := 1
		perPage := 1
		resp, err := setup.api.SearchAll(adminCtx(), apigen.SearchAllRequestObject{
			Params: apigen.SearchAllParams{
				Q:       "  needle.  ",
				DagPage: &page,
				DocPage: &docPage,
				PerPage: &perPage,
			},
		})
		require.NoError(t, err)

		searchResp, ok := resp.(apigen.SearchAll200JSONResponse)
		require.True(t, ok)

		require.Len(t, searchResp.Dags.Results, 1)
		assert.Equal(t, "b-match", searchResp.Dags.Results[0].FileName)
		assert.Equal(t, 2, searchResp.Dags.Pagination.TotalRecords)
		assert.Equal(t, 2, searchResp.Dags.Pagination.CurrentPage)

		require.Len(t, searchResp.Docs.Results, 1)
		assert.Equal(t, "alpha", searchResp.Docs.Results[0].Id)
		assert.Equal(t, 2, searchResp.Docs.Pagination.TotalRecords)

		firstDoc := searchResp.Docs.Results[0]
		assert.Equal(t, 4, firstDoc.MatchCount)
		assert.Len(t, firstDoc.Matches, 3)
		assert.True(t, firstDoc.HasMoreMatches)
	})

	t.Run("falls back to empty docs section when doc store is unavailable", func(t *testing.T) {
		t.Parallel()

		setup := newSearchTestSetup(t, false)
		mustCreateDAG(t, setup, "only-dag", `name: only-dag
steps:
  - command: echo "needle."`)

		resp, err := setup.api.SearchAll(adminCtx(), apigen.SearchAllRequestObject{
			Params: apigen.SearchAllParams{Q: "needle."},
		})
		require.NoError(t, err)

		searchResp := resp.(apigen.SearchAll200JSONResponse)
		assert.Len(t, searchResp.Dags.Results, 1)
		assert.Empty(t, searchResp.Docs.Results)
		assert.Equal(t, 0, searchResp.Docs.Pagination.TotalRecords)
	})
}

func TestSearchDagMatches(t *testing.T) {
	t.Parallel()

	setup := newSearchTestSetup(t, false)
	mustCreateDAG(t, setup, "match-heavy", `name: match-heavy
steps:
  - command: echo "needle."
  - command: echo "needle."
  - command: echo "needle."
  - command: echo "needle."
  - command: echo "needle."`)

	page := 2
	perPage := 3
	resp, err := setup.api.SearchDagMatches(adminCtx(), apigen.SearchDagMatchesRequestObject{
		FileName: "match-heavy",
		Params: apigen.SearchDagMatchesParams{
			Q:       "needle.",
			Page:    &page,
			PerPage: &perPage,
		},
	})
	require.NoError(t, err)

	matchResp, ok := resp.(apigen.SearchDagMatches200JSONResponse)
	require.True(t, ok)
	assert.Len(t, matchResp.Matches, 2)
	assert.Equal(t, 5, matchResp.Pagination.TotalRecords)
	assert.Equal(t, 2, matchResp.Pagination.CurrentPage)
}

func TestSearchDocMatches(t *testing.T) {
	t.Parallel()

	setup := newSearchTestSetup(t, true)
	mustCreateDoc(t, setup, "guides/runbook", "needle.\nneedle.\nneedle.\nneedle.")

	page := 2
	perPage := 3
	resp, err := setup.api.SearchDocMatches(adminCtx(), apigen.SearchDocMatchesRequestObject{
		Params: apigen.SearchDocMatchesParams{
			Path:    "guides/runbook",
			Q:       "needle.",
			Page:    &page,
			PerPage: &perPage,
		},
	})
	require.NoError(t, err)

	matchResp, ok := resp.(apigen.SearchDocMatches200JSONResponse)
	require.True(t, ok)
	assert.Len(t, matchResp.Matches, 1)
	assert.Equal(t, 4, matchResp.Pagination.TotalRecords)
	assert.Equal(t, 2, matchResp.Pagination.CurrentPage)
}
