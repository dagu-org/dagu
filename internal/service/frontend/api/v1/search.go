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
	searchDefaultLimit        = 20
	searchDefaultMatchLimit   = 5
	searchMaxLimit            = 50
	searchPreviewMatchesLimit = 1
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

func normalizeSearchLimit(limit int, defaultValue int) int {
	if limit <= 0 {
		limit = defaultValue
	}
	if limit > searchMaxLimit {
		limit = searchMaxLimit
	}
	return limit
}

func invalidSearchCursorError() error {
	return &Error{
		Code:       api.ErrorCodeBadRequest,
		Message:    "invalid search cursor",
		HTTPStatus: http.StatusBadRequest,
	}
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

func toDAGSearchFeedResponse(result *exec.CursorResult[exec.SearchDAGResult]) api.DAGSearchFeedResponse {
	items := make([]api.DAGSearchPageItem, 0, len(result.Items))
	for _, item := range result.Items {
		resp := api.DAGSearchPageItem{
			FileName:       item.FileName,
			Name:           item.FileName,
			HasMoreMatches: item.HasMoreMatches,
			Matches:        toSearchMatchItems(item.Matches),
		}
		if item.NextMatchesCursor != "" {
			resp.NextMatchesCursor = ptrOf(item.NextMatchesCursor)
		}
		items = append(items, resp)
	}

	response := api.DAGSearchFeedResponse{
		Results: items,
		HasMore: result.HasMore,
	}
	if result.NextCursor != "" {
		response.NextCursor = ptrOf(result.NextCursor)
	}
	return response
}

func toDocSearchFeedResponse(result *exec.CursorResult[agent.DocSearchResult]) api.DocSearchFeedResponse {
	items := make([]api.DocSearchPageItem, 0, len(result.Items))
	for _, item := range result.Items {
		resp := api.DocSearchPageItem{
			Id:             item.ID,
			Title:          item.Title,
			HasMoreMatches: item.HasMoreMatches,
			Matches:        toSearchMatchItems(item.Matches),
		}
		if item.NextMatchesCursor != "" {
			resp.NextMatchesCursor = ptrOf(item.NextMatchesCursor)
		}
		items = append(items, resp)
	}

	response := api.DocSearchFeedResponse{
		Results: items,
		HasMore: result.HasMore,
	}
	if result.NextCursor != "" {
		response.NextCursor = ptrOf(result.NextCursor)
	}
	return response
}

func toSearchMatchesResponse(result *exec.CursorResult[*exec.Match]) api.SearchMatchesResponse {
	response := api.SearchMatchesResponse{
		Matches: toSearchMatchItems(result.Items),
		HasMore: result.HasMore,
	}
	if result.NextCursor != "" {
		response.NextCursor = ptrOf(result.NextCursor)
	}
	return response
}

// SearchDAGFeed returns cursor-based DAG search results for the global search page.
func (a *API) SearchDAGFeed(ctx context.Context, request api.SearchDAGFeedRequestObject) (api.SearchDAGFeedResponseObject, error) {
	query, err := validateSearchQuery(request.Params.Q)
	if err != nil {
		return nil, err
	}

	result, errs, err := a.dagStore.SearchCursor(ctx, exec.SearchDAGsOptions{
		Cursor:     valueOf(request.Params.Cursor),
		Limit:      normalizeSearchLimit(valueOf(request.Params.Limit), searchDefaultLimit),
		Query:      query,
		MatchLimit: searchPreviewMatchesLimit,
	})
	if err != nil {
		if errors.Is(err, exec.ErrInvalidCursor) {
			return nil, invalidSearchCursorError()
		}
		logger.Error(ctx, "Failed to search DAGs", tag.Error(err))
		return nil, internalError(err)
	}
	for _, searchErr := range errs {
		logger.Warn(ctx, "Skipped DAG while searching", tag.Reason(searchErr))
	}

	return api.SearchDAGFeed200JSONResponse(toDAGSearchFeedResponse(result)), nil
}

// SearchDocFeed returns cursor-based document search results for the global search page.
func (a *API) SearchDocFeed(ctx context.Context, request api.SearchDocFeedRequestObject) (api.SearchDocFeedResponseObject, error) {
	query, err := validateSearchQuery(request.Params.Q)
	if err != nil {
		return nil, err
	}

	if a.docStore == nil {
		return api.SearchDocFeed200JSONResponse(api.DocSearchFeedResponse{
			Results: []api.DocSearchPageItem{},
			HasMore: false,
		}), nil
	}

	result, err := a.docStore.SearchCursor(ctx, agent.SearchDocsOptions{
		Cursor:     valueOf(request.Params.Cursor),
		Limit:      normalizeSearchLimit(valueOf(request.Params.Limit), searchDefaultLimit),
		Query:      query,
		MatchLimit: searchPreviewMatchesLimit,
	})
	if err != nil {
		if errors.Is(err, exec.ErrInvalidCursor) {
			return nil, invalidSearchCursorError()
		}
		logger.Error(ctx, "Failed to search docs", tag.Error(err))
		return nil, internalError(err)
	}

	return api.SearchDocFeed200JSONResponse(toDocSearchFeedResponse(result)), nil
}

// SearchDagMatches returns cursor-based snippets for one DAG result.
func (a *API) SearchDagMatches(ctx context.Context, request api.SearchDagMatchesRequestObject) (api.SearchDagMatchesResponseObject, error) {
	query, err := validateSearchQuery(request.Params.Q)
	if err != nil {
		return nil, err
	}

	result, err := a.dagStore.SearchMatches(ctx, request.FileName, exec.SearchDAGMatchesOptions{
		Cursor: valueOf(request.Params.Cursor),
		Limit:  normalizeSearchLimit(valueOf(request.Params.Limit), searchDefaultMatchLimit),
		Query:  query,
	})
	if err != nil {
		switch {
		case errors.Is(err, exec.ErrDAGNotFound):
			return nil, &Error{
				Code:       api.ErrorCodeNotFound,
				Message:    "DAG not found",
				HTTPStatus: http.StatusNotFound,
			}
		case errors.Is(err, exec.ErrInvalidCursor):
			return nil, invalidSearchCursorError()
		default:
			logger.Error(ctx, "Failed to search DAG matches", tag.Name(request.FileName), tag.Error(err))
			return nil, internalError(err)
		}
	}

	return api.SearchDagMatches200JSONResponse(toSearchMatchesResponse(result)), nil
}

// SearchDocMatches returns cursor-based snippets for one document result.
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

	result, err := a.docStore.SearchMatches(ctx, request.Params.Path, agent.SearchDocMatchesOptions{
		Cursor: valueOf(request.Params.Cursor),
		Limit:  normalizeSearchLimit(valueOf(request.Params.Limit), searchDefaultMatchLimit),
		Query:  query,
	})
	if err != nil {
		switch {
		case errors.Is(err, agent.ErrDocNotFound):
			return nil, errDocNotFound
		case errors.Is(err, exec.ErrInvalidCursor):
			return nil, invalidSearchCursorError()
		default:
			logger.Error(ctx, "Failed to search doc matches", tag.Name(request.Params.Path), tag.Error(err))
			return nil, internalError(err)
		}
	}

	return api.SearchDocMatches200JSONResponse(toSearchMatchesResponse(result)), nil
}
