// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dagucloud/dagu/api/v1"
	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/auth"
	"github.com/dagucloud/dagu/internal/cmn/logger"
	"github.com/dagucloud/dagu/internal/cmn/logger/tag"
	"github.com/dagucloud/dagu/internal/service/audit"
	"github.com/dagucloud/dagu/internal/workspace"
)

const (
	auditActionDocCreate = "doc_create"
	auditActionDocUpdate = "doc_update"
	auditActionDocDelete = "doc_delete"
	auditActionDocRename = "doc_rename"
)

var (
	errDocStoreNotAvailable = &Error{
		Code:       api.ErrorCodeForbidden,
		Message:    "Document management is not available",
		HTTPStatus: http.StatusForbidden,
	}

	errDocNotFound = &Error{
		Code:       api.ErrorCodeNotFound,
		Message:    "Document not found",
		HTTPStatus: http.StatusNotFound,
	}

	errDocAlreadyExists = &Error{
		Code:       api.ErrorCodeAlreadyExists,
		Message:    "Document already exists",
		HTTPStatus: http.StatusConflict,
	}
)

func (a *API) requireDocManagement() error {
	if a.docStore == nil {
		return errDocStoreNotAvailable
	}
	return nil
}

func validateDocPath(path string) error {
	if err := agent.ValidateDocID(path); err != nil {
		return &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    fmt.Sprintf("invalid doc path: %v", err),
			HTTPStatus: http.StatusBadRequest,
		}
	}
	return nil
}

func validateDocWorkspace(name *string) (string, error) {
	if name == nil || *name == "" {
		return "", nil
	}
	if err := workspace.ValidateName(*name); err != nil {
		return "", &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "invalid workspace: must contain only letters, numbers, underscores, and hyphens",
			HTTPStatus: http.StatusBadRequest,
		}
	}
	return *name, nil
}

func scopedDocPath(workspaceName, path string) (string, error) {
	if err := validateDocPath(path); err != nil {
		return "", err
	}
	if workspaceName == "" {
		return path, nil
	}
	scoped := workspaceName + "/" + path
	if err := validateDocPath(scoped); err != nil {
		return "", err
	}
	return scoped, nil
}

func visibleDocPath(workspaceName, path string) string {
	if workspaceName == "" {
		return path
	}
	return strings.TrimPrefix(path, workspaceName+"/")
}

type docWorkspaceVisibility struct {
	all     bool
	allowed map[string]struct{}
	known   map[string]struct{}
}

func (a *API) knownDocWorkspaceNames(ctx context.Context, required bool) (map[string]struct{}, error) {
	if a.workspaceStore == nil {
		if required {
			return nil, workspaceStoreUnavailable()
		}
		return nil, nil
	}
	workspaces, err := a.workspaceStore.List(ctx)
	if err != nil {
		if required {
			return nil, fmt.Errorf("failed to list workspaces: %w", err)
		}
		return nil, nil
	}
	known := make(map[string]struct{}, len(workspaces))
	for _, ws := range workspaces {
		known[ws.Name] = struct{}{}
	}
	return known, nil
}

func (a *API) docWorkspaceVisibility(ctx context.Context) (docWorkspaceVisibility, error) {
	visibility := docWorkspaceVisibility{all: true}
	known, _ := a.knownDocWorkspaceNames(ctx, false)
	visibility.known = known
	if a.authService == nil {
		return visibility, nil
	}
	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return visibility, errAuthRequired
	}
	access := auth.NormalizeWorkspaceAccess(user.WorkspaceAccess)
	if access.All {
		return visibility, nil
	}
	known, err := a.knownDocWorkspaceNames(ctx, true)
	if err != nil {
		return visibility, err
	}
	visibility.all = false
	visibility.allowed = make(map[string]struct{}, len(access.Grants))
	visibility.known = known
	for _, grant := range access.Grants {
		visibility.allowed[grant.Workspace] = struct{}{}
	}
	return visibility, nil
}

func (a *API) noWorkspaceDocVisibility(ctx context.Context) (docWorkspaceVisibility, error) {
	known, err := a.knownDocWorkspaceNames(ctx, true)
	if err != nil {
		return docWorkspaceVisibility{}, err
	}
	return docWorkspaceVisibility{
		allowed: make(map[string]struct{}),
		known:   known,
	}, nil
}

func (a *API) docWorkspaceVisibilityForSelection(ctx context.Context, selection workspaceScopeSelection) (docWorkspaceVisibility, error) {
	switch selection.scope {
	case api.WorkspaceScopeAccessible:
		return a.docWorkspaceVisibility(ctx)
	case api.WorkspaceScopeNone:
		return a.noWorkspaceDocVisibility(ctx)
	case api.WorkspaceScopeWorkspace:
		if err := a.requireWorkspaceVisible(ctx, selection.workspace); err != nil {
			return docWorkspaceVisibility{}, err
		}
		return docWorkspaceVisibility{all: true}, nil
	default:
		return docWorkspaceVisibility{}, badWorkspaceScopeError("invalid workspaceScope")
	}
}

func (a *API) docReadScopeForParams(
	ctx context.Context,
	scopeParam *api.WorkspaceScope,
	workspaceParam *api.Workspace,
) (string, docWorkspaceVisibility, error) {
	selection, err := parseWorkspaceScope(scopeParam, workspaceParam)
	if err != nil {
		return "", docWorkspaceVisibility{}, err
	}
	visibility, err := a.docWorkspaceVisibilityForSelection(ctx, selection)
	if err != nil {
		return "", docWorkspaceVisibility{}, err
	}
	if selection.scope == api.WorkspaceScopeWorkspace {
		return selection.workspace, visibility, nil
	}
	return "", visibility, nil
}

func docMutationScopeForParams(scopeParam *api.WorkspaceMutationScope, workspaceParam *api.Workspace) (string, error) {
	var readScope *api.WorkspaceScope
	if scopeParam != nil {
		converted := api.WorkspaceScope(*scopeParam)
		readScope = &converted
	}
	selection, err := parseWorkspaceScope(readScope, workspaceParam)
	if err != nil {
		return "", err
	}
	switch selection.scope {
	case api.WorkspaceScopeAccessible:
		if selection.explicit {
			return "", badWorkspaceScopeError("workspaceScope=accessible cannot be used for document mutations")
		}
		return "", nil
	case api.WorkspaceScopeNone:
		return "", nil
	case api.WorkspaceScopeWorkspace:
		return selection.workspace, nil
	default:
		return "", badWorkspaceScopeError("invalid workspaceScope")
	}
}

func (v docWorkspaceVisibility) knownWorkspace(name string) bool {
	if name == "" {
		return false
	}
	if v.known != nil {
		_, ok := v.known[name]
		return ok
	}
	if v.allowed != nil {
		_, ok := v.allowed[name]
		return ok
	}
	return false
}

func docWorkspaceNameForPath(path string, visibility docWorkspaceVisibility, includeWorkspaceRoot bool) string {
	workspaceName, rest, hasSlash := strings.Cut(path, "/")
	if workspaceName == "" {
		return ""
	}
	if !hasSlash && !includeWorkspaceRoot {
		return ""
	}
	if hasSlash && rest == "" {
		return ""
	}
	if visibility.knownWorkspace(workspaceName) {
		return workspaceName
	}
	return ""
}

func docWorkspaceValue(workspaceName, path string, visibility docWorkspaceVisibility, includeWorkspaceRoot bool) *string {
	if workspaceName != "" {
		return ptrOf(workspaceName)
	}
	return optionalString(docWorkspaceNameForPath(path, visibility, includeWorkspaceRoot))
}

func (v docWorkspaceVisibility) visible(path string) bool {
	if v.all {
		return true
	}
	workspaceName, _, _ := strings.Cut(path, "/")
	if workspaceName == "" {
		return true
	}
	if _, ok := v.known[workspaceName]; !ok {
		return true
	}
	_, ok := v.allowed[workspaceName]
	return ok
}

func filterDocMetadataByWorkspace(items []api.DocMetadataResponse, visibility docWorkspaceVisibility) []api.DocMetadataResponse {
	if visibility.all {
		return items
	}
	filtered := make([]api.DocMetadataResponse, 0, len(items))
	for _, item := range items {
		if visibility.visible(item.Id) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterDocTreeByWorkspace(nodes []api.DocTreeNodeResponse, visibility docWorkspaceVisibility) []api.DocTreeNodeResponse {
	if visibility.all {
		return nodes
	}
	filtered := make([]api.DocTreeNodeResponse, 0, len(nodes))
	for _, node := range nodes {
		if node.Children != nil {
			children := filterDocTreeByWorkspace(*node.Children, visibility)
			node.Children = &children
		}
		if visibility.visible(node.Id) {
			filtered = append(filtered, node)
		}
	}
	return filtered
}

// ListDocs returns documents as tree or flat list.
func (a *API) ListDocs(ctx context.Context, request api.ListDocsRequestObject) (api.ListDocsResponseObject, error) {
	if err := a.requireDocManagement(); err != nil {
		return nil, err
	}
	workspaceName, visibility, err := a.docReadScopeForParams(ctx, request.Params.WorkspaceScope, request.Params.Workspace)
	if err != nil {
		return nil, err
	}

	sortField, sortOrder := docSortParams(request.Params.Sort, request.Params.Order)

	opts := agent.ListDocsOptions{
		Page:       valueOf(request.Params.Page),
		PerPage:    valueOf(request.Params.PerPage),
		Sort:       sortField,
		Order:      sortOrder,
		PathPrefix: workspaceName,
	}

	flat := valueOf(request.Params.Flat)

	if flat {
		result, err := a.docStore.ListFlat(ctx, opts)
		if err != nil {
			logger.Error(ctx, "Failed to list docs flat", tag.Error(err))
			return nil, internalError(err)
		}

		items := make([]api.DocMetadataResponse, 0, len(result.Items))
		for _, m := range result.Items {
			item := toDocMetadataResponse(m)
			item.Workspace = docWorkspaceValue(workspaceName, m.ID, visibility, false)
			items = append(items, item)
		}
		items = filterDocMetadataByWorkspace(items, visibility)

		return api.ListDocs200JSONResponse{
			Items:      &items,
			Pagination: toPagination(*result),
		}, nil
	}

	result, err := a.docStore.List(ctx, opts)
	if err != nil {
		logger.Error(ctx, "Failed to list docs tree", tag.Error(err))
		return nil, internalError(err)
	}

	tree := make([]api.DocTreeNodeResponse, 0, len(result.Items))
	for _, node := range result.Items {
		tree = append(tree, toDocTreeResponseWithWorkspace(node, workspaceName, visibility))
	}
	tree = filterDocTreeByWorkspace(tree, visibility)

	return api.ListDocs200JSONResponse{
		Tree:       &tree,
		Pagination: toPagination(*result),
	}, nil
}

// CreateDoc creates a new document.
func (a *API) CreateDoc(ctx context.Context, request api.CreateDocRequestObject) (api.CreateDocResponseObject, error) {
	if err := a.requireDocManagement(); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	workspaceName, err := docMutationScopeForParams(request.Params.WorkspaceScope, request.Params.Workspace)
	if err != nil {
		return nil, err
	}
	if err := a.requireDAGWriteForWorkspace(ctx, workspaceName); err != nil {
		return nil, err
	}

	id := request.Body.Id
	scopedID, err := scopedDocPath(workspaceName, id)
	if err != nil {
		return nil, err
	}

	if err := a.docStore.Create(ctx, scopedID, request.Body.Content); err != nil {
		if errors.Is(err, agent.ErrDocAlreadyExists) {
			return nil, errDocAlreadyExists
		}
		logger.Error(ctx, "Failed to create doc", tag.Error(err))
		return nil, internalError(err)
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionDocCreate, map[string]any{
		"doc_id":    id,
		"workspace": workspaceName,
	})

	msg := fmt.Sprintf("Document %s created", id)
	return api.CreateDoc201JSONResponse{Message: &msg}, nil
}

// GetDoc returns a single document.
func (a *API) GetDoc(ctx context.Context, request api.GetDocRequestObject) (api.GetDocResponseObject, error) {
	if err := a.requireDocManagement(); err != nil {
		return nil, err
	}
	workspaceName, visibility, err := a.docReadScopeForParams(ctx, request.Params.WorkspaceScope, request.Params.Workspace)
	if err != nil {
		return nil, err
	}
	docID, err := scopedDocPath(workspaceName, request.Params.Path)
	if err != nil {
		return nil, err
	}
	doc, err := a.docStore.Get(ctx, docID)
	if err != nil {
		if errors.Is(err, agent.ErrDocNotFound) {
			return nil, errDocNotFound
		}
		return nil, internalError(err)
	}
	if workspaceName == "" && !visibility.all {
		if !visibility.visible(doc.ID) {
			return nil, errDocNotFound
		}
	}
	rawID := doc.ID
	doc.ID = visibleDocPath(workspaceName, doc.ID)
	resp := toDocResponse(doc)
	resp.Workspace = docWorkspaceValue(workspaceName, rawID, visibility, false)

	return api.GetDoc200JSONResponse(resp), nil
}

// SearchDocs searches document content.
func (a *API) SearchDocs(ctx context.Context, request api.SearchDocsRequestObject) (api.SearchDocsResponseObject, error) {
	if err := a.requireDocManagement(); err != nil {
		return nil, err
	}

	if request.Params.Q == "" {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "query parameter 'q' is required",
			HTTPStatus: http.StatusBadRequest,
		}
	}
	workspaceName, visibility, err := a.docReadScopeForParams(ctx, request.Params.WorkspaceScope, request.Params.Workspace)
	if err != nil {
		return nil, err
	}

	results, err := a.docStore.Search(ctx, request.Params.Q)
	if err != nil {
		logger.Error(ctx, "Failed to search docs", tag.Error(err))
		return nil, internalError(err)
	}

	items := make([]api.DocSearchResultItem, 0, len(results))
	for _, r := range results {
		rawID := r.ID
		if workspaceName != "" {
			prefix := workspaceName + "/"
			if !strings.HasPrefix(r.ID, prefix) {
				continue
			}
			r.ID = strings.TrimPrefix(r.ID, prefix)
		} else if !visibility.visible(r.ID) {
			continue
		}
		item := api.DocSearchResultItem{
			Id:        r.ID,
			Title:     r.Title,
			Workspace: docWorkspaceValue(workspaceName, rawID, visibility, false),
		}
		if len(r.Matches) > 0 {
			matches := make([]api.SearchMatchItem, 0, len(r.Matches))
			for _, m := range r.Matches {
				matches = append(matches, api.SearchMatchItem{
					Line:       m.Line,
					LineNumber: m.LineNumber,
					StartLine:  m.StartLine,
				})
			}
			item.Matches = &matches
		}
		items = append(items, item)
	}

	return api.SearchDocs200JSONResponse{
		Results: items,
	}, nil
}

// UpdateDoc updates document content.
func (a *API) UpdateDoc(ctx context.Context, request api.UpdateDocRequestObject) (api.UpdateDocResponseObject, error) {
	if err := a.requireDocManagement(); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	workspaceName, err := docMutationScopeForParams(request.Params.WorkspaceScope, request.Params.Workspace)
	if err != nil {
		return nil, err
	}
	if err := a.requireDAGWriteForWorkspace(ctx, workspaceName); err != nil {
		return nil, err
	}
	docID, err := scopedDocPath(workspaceName, request.Params.Path)
	if err != nil {
		return nil, err
	}
	if err := a.docStore.Update(ctx, docID, request.Body.Content); err != nil {
		if errors.Is(err, agent.ErrDocNotFound) {
			return nil, errDocNotFound
		}
		logger.Error(ctx, "Failed to update doc", tag.Error(err))
		return nil, internalError(err)
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionDocUpdate, map[string]any{
		"doc_id":    request.Params.Path,
		"workspace": workspaceName,
	})

	msg := "Document updated"
	return api.UpdateDoc200JSONResponse{Message: &msg}, nil
}

// DeleteDoc removes a document.
func (a *API) DeleteDoc(ctx context.Context, request api.DeleteDocRequestObject) (api.DeleteDocResponseObject, error) {
	if err := a.requireDocManagement(); err != nil {
		return nil, err
	}
	workspaceName, err := docMutationScopeForParams(request.Params.WorkspaceScope, request.Params.Workspace)
	if err != nil {
		return nil, err
	}
	if err := a.requireDAGWriteForWorkspace(ctx, workspaceName); err != nil {
		return nil, err
	}
	docID, err := scopedDocPath(workspaceName, request.Params.Path)
	if err != nil {
		return nil, err
	}

	if err := a.docStore.Delete(ctx, docID); err != nil {
		if errors.Is(err, agent.ErrDocNotFound) {
			return nil, errDocNotFound
		}
		logger.Error(ctx, "Failed to delete doc", tag.Error(err))
		return nil, internalError(err)
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionDocDelete, map[string]any{
		"doc_id":    request.Params.Path,
		"workspace": workspaceName,
	})

	return api.DeleteDoc204Response{}, nil
}

// RenameDoc renames/moves a document.
func (a *API) RenameDoc(ctx context.Context, request api.RenameDocRequestObject) (api.RenameDocResponseObject, error) {
	if err := a.requireDocManagement(); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	workspaceName, err := docMutationScopeForParams(request.Params.WorkspaceScope, request.Params.Workspace)
	if err != nil {
		return nil, err
	}
	if err := a.requireDAGWriteForWorkspace(ctx, workspaceName); err != nil {
		return nil, err
	}
	oldPath, err := scopedDocPath(workspaceName, request.Params.Path)
	if err != nil {
		return nil, err
	}
	newPath, err := scopedDocPath(workspaceName, request.Body.NewPath)
	if err != nil {
		return nil, err
	}

	if err := a.docStore.Rename(ctx, oldPath, newPath); err != nil {
		if errors.Is(err, agent.ErrDocNotFound) {
			return nil, errDocNotFound
		}
		if errors.Is(err, agent.ErrDocAlreadyExists) {
			return nil, errDocAlreadyExists
		}
		logger.Error(ctx, "Failed to rename doc", tag.Error(err))
		return nil, internalError(err)
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionDocRename, map[string]any{
		"old_path":  request.Params.Path,
		"new_path":  request.Body.NewPath,
		"workspace": workspaceName,
	})

	msg := fmt.Sprintf("Document renamed to %s", request.Body.NewPath)
	return api.RenameDoc200JSONResponse{Message: &msg}, nil
}

// DeleteDocBatch deletes multiple documents or directories.
func (a *API) DeleteDocBatch(ctx context.Context, request api.DeleteDocBatchRequestObject) (api.DeleteDocBatchResponseObject, error) {
	if err := a.requireDocManagement(); err != nil {
		return nil, err
	}
	if request.Body == nil || len(request.Body.Paths) == 0 {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "paths required",
			HTTPStatus: http.StatusBadRequest,
		}
	}
	if len(request.Body.Paths) > 100 {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "max 100 paths per batch",
			HTTPStatus: http.StatusBadRequest,
		}
	}
	workspaceName, err := docMutationScopeForParams(request.Params.WorkspaceScope, request.Params.Workspace)
	if err != nil {
		return nil, err
	}
	if err := a.requireDAGWriteForWorkspace(ctx, workspaceName); err != nil {
		return nil, err
	}
	scopedPaths := make([]string, 0, len(request.Body.Paths))
	for _, p := range request.Body.Paths {
		scoped, err := scopedDocPath(workspaceName, p)
		if err != nil {
			return nil, err
		}
		scopedPaths = append(scopedPaths, scoped)
	}

	deleted, failed, err := a.docStore.DeleteBatch(ctx, scopedPaths)
	if err != nil {
		logger.Error(ctx, "Failed to batch delete docs", tag.Error(err))
		return nil, internalError(err)
	}

	visibleDeleted := make([]string, 0, len(deleted))
	for _, id := range deleted {
		visibleID := visibleDocPath(workspaceName, id)
		visibleDeleted = append(visibleDeleted, visibleID)
		a.logAudit(ctx, audit.CategoryAgent, auditActionDocDelete, map[string]any{
			"doc_id":    visibleID,
			"workspace": workspaceName,
		})
	}

	failedItems := make([]api.DocDeleteBatchFailedItem, 0, len(failed))
	for _, f := range failed {
		failedItems = append(failedItems, api.DocDeleteBatchFailedItem{
			Path:  visibleDocPath(workspaceName, f.ID),
			Error: f.Error,
		})
	}

	msg := fmt.Sprintf("Deleted %d, failed %d", len(visibleDeleted), len(failed))
	return api.DeleteDocBatch200JSONResponse{
		Deleted: visibleDeleted,
		Failed:  failedItems,
		Message: msg,
	}, nil
}

// GetDocTreeData is the SSE data method for the doc tree.
// Identifier format: URL query string (e.g., "page=1&perPage=200")
func (a *API) GetDocTreeData(ctx context.Context, queryString string) (any, error) {
	if a.docStore == nil {
		return nil, errDocStoreNotAvailable
	}

	params, err := url.ParseQuery(queryString)
	if err != nil {
		params = url.Values{}
	}

	page := parseIntParam(params.Get("page"), 1)
	perPage := min(parseIntParam(params.Get("perPage"), 200), 200)
	scopeParam, workspaceParam := workspaceScopeParamsFromValues(params)
	workspaceName, visibility, err := a.docReadScopeForParams(ctx, scopeParam, workspaceParam)
	if err != nil {
		return nil, err
	}

	sortField, sortOrder := docSortParamsFromQuery(params)

	result, err := a.docStore.List(ctx, agent.ListDocsOptions{
		Page:       page,
		PerPage:    perPage,
		Sort:       sortField,
		Order:      sortOrder,
		PathPrefix: workspaceName,
	})
	if err != nil {
		return nil, err
	}

	tree := make([]api.DocTreeNodeResponse, 0, len(result.Items))
	for _, node := range result.Items {
		tree = append(tree, toDocTreeResponseWithWorkspace(node, workspaceName, visibility))
	}
	tree = filterDocTreeByWorkspace(tree, visibility)

	return api.ListDocs200JSONResponse{
		Tree:       &tree,
		Pagination: toPagination(*result),
	}, nil
}

// GetDocContentData is the SSE data method for doc content.
func (a *API) GetDocContentData(ctx context.Context, docID string) (any, error) {
	if a.docStore == nil {
		return nil, errDocStoreNotAvailable
	}
	path, queryString, hasQuery := strings.Cut(docID, "?")
	workspaceName, visibility, err := a.docReadScopeForParams(ctx, nil, nil)
	if err != nil {
		return nil, err
	}
	if hasQuery {
		params, err := url.ParseQuery(queryString)
		if err != nil {
			return nil, err
		}
		scopeParam, workspaceParam := workspaceScopeParamsFromValues(params)
		workspaceName, visibility, err = a.docReadScopeForParams(ctx, scopeParam, workspaceParam)
		if err != nil {
			return nil, err
		}
	}
	scopedID, err := scopedDocPath(workspaceName, path)
	if err != nil {
		return nil, err
	}
	doc, err := a.docStore.Get(ctx, scopedID)
	if err != nil {
		return nil, err
	}
	if workspaceName == "" && !visibility.all {
		if !visibility.visible(doc.ID) {
			return nil, errDocNotFound
		}
	}
	rawID := doc.ID
	doc.ID = visibleDocPath(workspaceName, doc.ID)
	resp := toDocResponse(doc)
	resp.Workspace = docWorkspaceValue(workspaceName, rawID, visibility, false)
	return resp, nil
}

func toDocResponse(doc *agent.Doc) api.DocResponse {
	resp := api.DocResponse{
		Id:      doc.ID,
		Title:   doc.Title,
		Content: doc.Content,
	}
	if doc.FilePath != "" {
		resp.FilePath = &doc.FilePath
	}
	if t, err := time.Parse(time.RFC3339, doc.CreatedAt); err == nil {
		resp.CreatedAt = &t
	}
	if t, err := time.Parse(time.RFC3339, doc.UpdatedAt); err == nil {
		resp.UpdatedAt = &t
	}
	return resp
}

func toDocMetadataResponse(m agent.DocMetadata) api.DocMetadataResponse {
	resp := api.DocMetadataResponse{
		Id:    m.ID,
		Title: m.Title,
	}
	if !m.ModTime.IsZero() {
		t := m.ModTime
		resp.ModifiedAt = &t
	}
	return resp
}

func toDocTreeResponse(node *agent.DocTreeNode) api.DocTreeNodeResponse {
	return toDocTreeResponseWithWorkspace(node, "", docWorkspaceVisibility{})
}

func toDocTreeResponseWithWorkspace(
	node *agent.DocTreeNode,
	workspaceName string,
	visibility docWorkspaceVisibility,
) api.DocTreeNodeResponse {
	resp := api.DocTreeNodeResponse{
		Id:        node.ID,
		Name:      node.Name,
		Title:     ptrOf(node.Title),
		Type:      api.DocTreeNodeResponseType(node.Type),
		Workspace: docWorkspaceValue(workspaceName, node.ID, visibility, node.Type == "directory"),
	}
	if !node.ModTime.IsZero() {
		t := node.ModTime
		resp.ModifiedAt = &t
	}
	if len(node.Children) > 0 {
		children := make([]api.DocTreeNodeResponse, 0, len(node.Children))
		for _, child := range node.Children {
			children = append(children, toDocTreeResponseWithWorkspace(child, workspaceName, visibility))
		}
		resp.Children = &children
	}
	return resp
}

// docSortParams extracts sort field and order from typed request params with defaults.
func docSortParams(sort *api.ListDocsParamsSort, order *api.ListDocsParamsOrder) (agent.DocSortField, agent.DocSortOrder) {
	s := agent.DocSortFieldType
	if sort != nil {
		s = agent.DocSortField(*sort)
	}
	o := agent.DocSortOrderAsc
	if order != nil {
		o = agent.DocSortOrder(*order)
	}
	return s, o
}

// docSortParamsFromQuery extracts sort field and order from URL query values
// with validation. Invalid values fall back to defaults.
func docSortParamsFromQuery(params url.Values) (agent.DocSortField, agent.DocSortOrder) {
	s := agent.DocSortField(params.Get("sort"))
	switch s {
	case agent.DocSortFieldName, agent.DocSortFieldType, agent.DocSortFieldMTime:
		// valid
	default:
		s = agent.DocSortFieldType
	}
	o := agent.DocSortOrder(params.Get("order"))
	switch o {
	case agent.DocSortOrderAsc, agent.DocSortOrderDesc:
		// valid
	default:
		o = agent.DocSortOrderAsc
	}
	return s, o
}
