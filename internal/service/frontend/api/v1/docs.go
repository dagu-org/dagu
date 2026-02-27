package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/service/audit"
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

// ListDocs returns documents as tree or flat list.
func (a *API) ListDocs(ctx context.Context, request api.ListDocsRequestObject) (api.ListDocsResponseObject, error) {
	if err := a.requireDocManagement(); err != nil {
		return nil, err
	}

	page := valueOf(request.Params.Page)
	perPage := valueOf(request.Params.PerPage)
	flat := valueOf(request.Params.Flat)

	if flat {
		result, err := a.docStore.ListFlat(ctx, page, perPage)
		if err != nil {
			logger.Error(ctx, "Failed to list docs flat", tag.Error(err))
			return nil, internalError(err)
		}

		items := make([]api.DocMetadataResponse, 0, len(result.Items))
		for _, m := range result.Items {
			items = append(items, toDocMetadataResponse(m))
		}

		return api.ListDocs200JSONResponse{
			Items:      &items,
			Pagination: ptrOf(toPagination(*result)),
		}, nil
	}

	result, err := a.docStore.List(ctx, page, perPage)
	if err != nil {
		logger.Error(ctx, "Failed to list docs tree", tag.Error(err))
		return nil, internalError(err)
	}

	tree := make([]api.DocTreeNodeResponse, 0, len(result.Items))
	for _, node := range result.Items {
		tree = append(tree, toDocTreeResponse(node))
	}

	return api.ListDocs200JSONResponse{
		Tree:       &tree,
		Pagination: ptrOf(toPagination(*result)),
	}, nil
}

// CreateDoc creates a new document.
func (a *API) CreateDoc(ctx context.Context, request api.CreateDocRequestObject) (api.CreateDocResponseObject, error) {
	if err := a.requireDocManagement(); err != nil {
		return nil, err
	}
	if err := a.requireDAGWrite(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}

	id := request.Body.Id
	if err := validateDocPath(id); err != nil {
		return nil, err
	}

	if err := a.docStore.Create(ctx, id, request.Body.Content); err != nil {
		if errors.Is(err, agent.ErrDocAlreadyExists) {
			return nil, errDocAlreadyExists
		}
		logger.Error(ctx, "Failed to create doc", tag.Error(err))
		return nil, internalError(err)
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionDocCreate, map[string]any{
		"doc_id": id,
	})

	msg := fmt.Sprintf("Document %s created", id)
	return api.CreateDoc201JSONResponse{Message: &msg}, nil
}

// GetDoc returns a single document.
func (a *API) GetDoc(ctx context.Context, request api.GetDocRequestObject) (api.GetDocResponseObject, error) {
	if err := a.requireDocManagement(); err != nil {
		return nil, err
	}
	if err := validateDocPath(request.Params.Path); err != nil {
		return nil, err
	}

	doc, err := a.docStore.Get(ctx, request.Params.Path)
	if err != nil {
		if errors.Is(err, agent.ErrDocNotFound) {
			return nil, errDocNotFound
		}
		return nil, internalError(err)
	}

	return api.GetDoc200JSONResponse(toDocResponse(doc)), nil
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

	results, err := a.docStore.Search(ctx, request.Params.Q)
	if err != nil {
		logger.Error(ctx, "Failed to search docs", tag.Error(err))
		return nil, internalError(err)
	}

	items := make([]api.DocSearchResultItem, 0, len(results))
	for _, r := range results {
		item := api.DocSearchResultItem{
			Id:    r.ID,
			Title: r.Title,
		}
		if len(r.Matches) > 0 {
			matches := make([]api.SearchDAGsMatchItem, 0, len(r.Matches))
			for _, m := range r.Matches {
				matches = append(matches, api.SearchDAGsMatchItem{
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
		Results: &items,
	}, nil
}

// UpdateDoc updates document content.
func (a *API) UpdateDoc(ctx context.Context, request api.UpdateDocRequestObject) (api.UpdateDocResponseObject, error) {
	if err := a.requireDocManagement(); err != nil {
		return nil, err
	}
	if err := a.requireDAGWrite(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	if err := validateDocPath(request.Params.Path); err != nil {
		return nil, err
	}

	if err := a.docStore.Update(ctx, request.Params.Path, request.Body.Content); err != nil {
		if errors.Is(err, agent.ErrDocNotFound) {
			return nil, errDocNotFound
		}
		logger.Error(ctx, "Failed to update doc", tag.Error(err))
		return nil, internalError(err)
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionDocUpdate, map[string]any{
		"doc_id": request.Params.Path,
	})

	msg := "Document updated"
	return api.UpdateDoc200JSONResponse{Message: &msg}, nil
}

// DeleteDoc removes a document.
func (a *API) DeleteDoc(ctx context.Context, request api.DeleteDocRequestObject) (api.DeleteDocResponseObject, error) {
	if err := a.requireDocManagement(); err != nil {
		return nil, err
	}
	if err := a.requireDAGWrite(ctx); err != nil {
		return nil, err
	}
	if err := validateDocPath(request.Params.Path); err != nil {
		return nil, err
	}

	if err := a.docStore.Delete(ctx, request.Params.Path); err != nil {
		if errors.Is(err, agent.ErrDocNotFound) {
			return nil, errDocNotFound
		}
		logger.Error(ctx, "Failed to delete doc", tag.Error(err))
		return nil, internalError(err)
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionDocDelete, map[string]any{
		"doc_id": request.Params.Path,
	})

	return api.DeleteDoc204Response{}, nil
}

// RenameDoc renames/moves a document.
func (a *API) RenameDoc(ctx context.Context, request api.RenameDocRequestObject) (api.RenameDocResponseObject, error) {
	if err := a.requireDocManagement(); err != nil {
		return nil, err
	}
	if err := a.requireDAGWrite(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}
	if err := validateDocPath(request.Params.Path); err != nil {
		return nil, err
	}
	if err := validateDocPath(request.Body.NewPath); err != nil {
		return nil, err
	}

	if err := a.docStore.Rename(ctx, request.Params.Path, request.Body.NewPath); err != nil {
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
		"old_path": request.Params.Path,
		"new_path": request.Body.NewPath,
	})

	msg := fmt.Sprintf("Document renamed to %s", request.Body.NewPath)
	return api.RenameDoc200JSONResponse{Message: &msg}, nil
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
	perPage := parseIntParam(params.Get("perPage"), 200)

	result, err := a.docStore.List(ctx, page, perPage)
	if err != nil {
		return nil, err
	}

	tree := make([]api.DocTreeNodeResponse, 0, len(result.Items))
	for _, node := range result.Items {
		tree = append(tree, toDocTreeResponse(node))
	}

	return api.ListDocs200JSONResponse{
		Tree:       &tree,
		Pagination: ptrOf(toPagination(*result)),
	}, nil
}

// GetDocContentData is the SSE data method for doc content.
func (a *API) GetDocContentData(ctx context.Context, docID string) (any, error) {
	if a.docStore == nil {
		return nil, errDocStoreNotAvailable
	}
	doc, err := a.docStore.Get(ctx, docID)
	if err != nil {
		return nil, err
	}
	return toDocResponse(doc), nil
}

func toDocResponse(doc *agent.Doc) api.DocResponse {
	return api.DocResponse{
		Id:        doc.ID,
		Title:     doc.Title,
		Content:   doc.Content,
		CreatedAt: ptrOf(doc.CreatedAt),
		UpdatedAt: ptrOf(doc.UpdatedAt),
	}
}

func toDocMetadataResponse(m agent.DocMetadata) api.DocMetadataResponse {
	return api.DocMetadataResponse{
		Id:    m.ID,
		Title: m.Title,
	}
}

func toDocTreeResponse(node *agent.DocTreeNode) api.DocTreeNodeResponse {
	resp := api.DocTreeNodeResponse{
		Id:    node.ID,
		Name:  node.Name,
		Title: ptrOf(node.Title),
		Type:  api.DocTreeNodeResponseType(node.Type),
	}
	if len(node.Children) > 0 {
		children := make([]api.DocTreeNodeResponse, 0, len(node.Children))
		for _, child := range node.Children {
			children = append(children, toDocTreeResponse(child))
		}
		resp.Children = &children
	}
	return resp
}
