// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	api "github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core/exec"
)

const (
	searchDefaultPerPage      = 10
	searchMaxPerPage          = 25
	searchPreviewMatchesLimit = 3
	searchMatchDefaultPerPage = 3
)

func validateSearchQuery(query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "query parameter 'q' is required",
			HTTPStatus: http.StatusBadRequest,
		}
	}
	return query, nil
}

func newSearchPaginator(page *int, perPage *int, defaultPerPage int) exec.Paginator {
	pageValue := valueOf(page)
	perPageValue := valueOf(perPage)
	if perPageValue <= 0 {
		perPageValue = defaultPerPage
	}
	if perPageValue > searchMaxPerPage {
		perPageValue = searchMaxPerPage
	}
	return exec.NewPaginator(pageValue, perPageValue)
}

func emptyPagination(pg exec.Paginator) api.Pagination {
	result := exec.NewPaginatedResult([]int{}, 0, pg)
	return toPagination(result)
}

func toSearchMatchItems(matches []*exec.Match) []api.SearchDAGsMatchItem {
	items := make([]api.SearchDAGsMatchItem, 0, len(matches))
	for _, match := range matches {
		items = append(items, api.SearchDAGsMatchItem{
			Line:       match.Line,
			LineNumber: match.LineNumber,
			StartLine:  match.StartLine,
		})
	}
	return items
}

func hasMoreMatches(total int, matches []*exec.Match) bool {
	return total > len(matches)
}

func toDAGSearchSection(results *exec.PaginatedResult[exec.SearchDAGResult]) api.DAGSearchSection {
	items := make([]api.DAGSearchPageItem, 0, len(results.Items))
	for _, item := range results.Items {
		items = append(items, api.DAGSearchPageItem{
			FileName:       item.FileName,
			Name:           item.FileName,
			MatchCount:     item.MatchCount,
			HasMoreMatches: hasMoreMatches(item.MatchCount, item.Matches),
			Matches:        toSearchMatchItems(item.Matches),
		})
	}

	return api.DAGSearchSection{
		Results:    items,
		Pagination: toPagination(*results),
	}
}

func toDocSearchSection(results *exec.PaginatedResult[agent.DocSearchResult]) api.DocSearchSection {
	items := make([]api.DocSearchPageItem, 0, len(results.Items))
	for _, item := range results.Items {
		items = append(items, api.DocSearchPageItem{
			Id:             item.ID,
			Title:          item.Title,
			MatchCount:     item.MatchCount,
			HasMoreMatches: hasMoreMatches(item.MatchCount, item.Matches),
			Matches:        toSearchMatchItems(item.Matches),
		})
	}

	return api.DocSearchSection{
		Results:    items,
		Pagination: toPagination(*results),
	}
}

// SearchAll returns paginated DAG and document search results for the global search page.
func (a *API) SearchAll(ctx context.Context, request api.SearchAllRequestObject) (api.SearchAllResponseObject, error) {
	query, err := validateSearchQuery(request.Params.Q)
	if err != nil {
		return nil, err
	}

	dagPaginator := newSearchPaginator(request.Params.DagPage, request.Params.PerPage, searchDefaultPerPage)
	docPaginator := newSearchPaginator(request.Params.DocPage, request.Params.PerPage, searchDefaultPerPage)

	dagResults, dagErrs, err := a.dagStore.SearchPaginated(ctx, exec.SearchDAGsOptions{
		Paginator:  dagPaginator,
		Query:      query,
		MatchLimit: searchPreviewMatchesLimit,
	})
	if err != nil {
		logger.Error(ctx, "Failed to search DAGs", tag.Error(err))
		return nil, internalError(err)
	}
	for _, dagErr := range dagErrs {
		logger.Warn(ctx, "Skipped DAG while searching", tag.Reason(dagErr))
	}

	docSection := api.DocSearchSection{
		Results:    []api.DocSearchPageItem{},
		Pagination: emptyPagination(docPaginator),
	}

	if a.docStore != nil {
		docResults, err := a.docStore.SearchPaginated(ctx, agent.SearchDocsOptions{
			Paginator:  docPaginator,
			Query:      query,
			MatchLimit: searchPreviewMatchesLimit,
		})
		if err != nil {
			logger.Error(ctx, "Failed to search docs", tag.Error(err))
			return nil, internalError(err)
		}
		docSection = toDocSearchSection(docResults)
	}

	return api.SearchAll200JSONResponse{
		Dags: toDAGSearchSection(dagResults),
		Docs: docSection,
	}, nil
}

// SearchDagMatches returns paginated snippets for one DAG result.
func (a *API) SearchDagMatches(ctx context.Context, request api.SearchDagMatchesRequestObject) (api.SearchDagMatchesResponseObject, error) {
	query, err := validateSearchQuery(request.Params.Q)
	if err != nil {
		return nil, err
	}

	paginator := newSearchPaginator(request.Params.Page, request.Params.PerPage, searchMatchDefaultPerPage)
	result, err := a.dagStore.SearchMatches(ctx, request.FileName, exec.SearchDAGMatchesOptions{
		Paginator: paginator,
		Query:     query,
	})
	if err != nil {
		if errors.Is(err, exec.ErrDAGNotFound) {
			return nil, &Error{
				Code:       api.ErrorCodeNotFound,
				Message:    "DAG not found",
				HTTPStatus: http.StatusNotFound,
			}
		}
		logger.Error(ctx, "Failed to search DAG matches", tag.Name(request.FileName), tag.Error(err))
		return nil, internalError(err)
	}

	return api.SearchDagMatches200JSONResponse{
		Matches:    toSearchMatchItems(result.Items),
		Pagination: toPagination(*result),
	}, nil
}

// SearchDocMatches returns paginated snippets for one document result.
func (a *API) SearchDocMatches(ctx context.Context, request api.SearchDocMatchesRequestObject) (api.SearchDocMatchesResponseObject, error) {
	if err := a.requireDocManagement(); err != nil {
		return nil, err
	}
	if err := validateDocPath(request.Params.Path); err != nil {
		return nil, err
	}

	query, err := validateSearchQuery(request.Params.Q)
	if err != nil {
		return nil, err
	}

	paginator := newSearchPaginator(request.Params.Page, request.Params.PerPage, searchMatchDefaultPerPage)
	result, err := a.docStore.SearchMatches(ctx, request.Params.Path, agent.SearchDocMatchesOptions{
		Paginator: paginator,
		Query:     query,
	})
	if err != nil {
		if errors.Is(err, agent.ErrDocNotFound) {
			return nil, errDocNotFound
		}
		logger.Error(ctx, "Failed to search doc matches", tag.Name(request.Params.Path), tag.Error(err))
		return nil, internalError(err)
	}

	return api.SearchDocMatches200JSONResponse{
		Matches:    toSearchMatchItems(result.Items),
		Pagination: toPagination(*result),
	}, nil
}
