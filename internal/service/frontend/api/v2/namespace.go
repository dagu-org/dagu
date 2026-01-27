package api

import (
	"context"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/persis/filens"
)

// ListNamespaces returns a list of all namespaces.
func (a *API) ListNamespaces(ctx context.Context, _ api.ListNamespacesRequestObject) (api.ListNamespacesResponseObject, error) {
	if a.namespaceStore == nil {
		// Namespace support not enabled, return empty list
		return api.ListNamespaces200JSONResponse{
			Namespaces: []api.NamespaceInfo{},
		}, nil
	}

	namespaces, err := a.namespaceStore.List(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]api.NamespaceInfo, 0, len(namespaces))
	for _, ns := range namespaces {
		result = append(result, toNamespaceInfo(ns))
	}

	return api.ListNamespaces200JSONResponse{
		Namespaces: result,
	}, nil
}

// CreateNamespace creates a new namespace.
func (a *API) CreateNamespace(ctx context.Context, request api.CreateNamespaceRequestObject) (api.CreateNamespaceResponseObject, error) {
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if a.namespaceStore == nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Namespace support not enabled",
			HTTPStatus: 500,
		}
	}

	if request.Body == nil {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "Request body is required",
			HTTPStatus: 400,
		}
	}

	ns := &filens.Namespace{
		Name:        request.Body.Name,
		DisplayName: valueOf(request.Body.DisplayName),
		Description: valueOf(request.Body.Description),
	}

	if err := a.namespaceStore.Create(ctx, ns); err != nil {
		if err == filens.ErrNamespaceAlreadyExists {
			return nil, &Error{
				Code:       api.ErrorCodeAlreadyExists,
				Message:    "Namespace already exists",
				HTTPStatus: 400,
			}
		}
		return nil, err
	}

	return api.CreateNamespace201JSONResponse(toNamespaceInfo(ns)), nil
}

// GetNamespace returns details of a specific namespace.
func (a *API) GetNamespace(ctx context.Context, request api.GetNamespaceRequestObject) (api.GetNamespaceResponseObject, error) {
	if a.namespaceStore == nil {
		return nil, &Error{
			Code:       api.ErrorCodeNotFound,
			Message:    "Namespace support not enabled",
			HTTPStatus: 404,
		}
	}

	ns, err := a.namespaceStore.GetByName(ctx, request.NamespaceName)
	if err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeNotFound,
			Message:    "Namespace not found",
			HTTPStatus: 404,
		}
	}

	return api.GetNamespace200JSONResponse(toNamespaceInfo(ns)), nil
}

// UpdateNamespace updates namespace metadata.
func (a *API) UpdateNamespace(ctx context.Context, request api.UpdateNamespaceRequestObject) (api.UpdateNamespaceResponseObject, error) {
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if a.namespaceStore == nil {
		return nil, &Error{
			Code:       api.ErrorCodeNotFound,
			Message:    "Namespace support not enabled",
			HTTPStatus: 404,
		}
	}

	if request.Body == nil {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "Request body is required",
			HTTPStatus: 400,
		}
	}

	ns, err := a.namespaceStore.GetByName(ctx, request.NamespaceName)
	if err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeNotFound,
			Message:    "Namespace not found",
			HTTPStatus: 404,
		}
	}

	// Update fields
	if request.Body.DisplayName != nil {
		ns.DisplayName = *request.Body.DisplayName
	}
	if request.Body.Description != nil {
		ns.Description = *request.Body.Description
	}

	if err := a.namespaceStore.Update(ctx, ns); err != nil {
		return nil, err
	}

	return api.UpdateNamespace200JSONResponse(toNamespaceInfo(ns)), nil
}

// DeleteNamespace deletes a namespace.
func (a *API) DeleteNamespace(ctx context.Context, request api.DeleteNamespaceRequestObject) (api.DeleteNamespaceResponseObject, error) {
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if a.namespaceStore == nil {
		return nil, &Error{
			Code:       api.ErrorCodeNotFound,
			Message:    "Namespace support not enabled",
			HTTPStatus: 404,
		}
	}

	ns, err := a.namespaceStore.GetByName(ctx, request.NamespaceName)
	if err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeNotFound,
			Message:    "Namespace not found",
			HTTPStatus: 404,
		}
	}

	// Prevent deleting the default namespace
	if ns.Name == filens.DefaultNamespaceName {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "Cannot delete the default namespace",
			HTTPStatus: 400,
		}
	}

	// TODO: Check if namespace is empty (no DAGs) before deleting

	if err := a.namespaceStore.Delete(ctx, ns.ID); err != nil {
		return nil, err
	}

	return api.DeleteNamespace204Response{}, nil
}

// RenameNamespace renames a namespace.
func (a *API) RenameNamespace(ctx context.Context, request api.RenameNamespaceRequestObject) (api.RenameNamespaceResponseObject, error) {
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if a.namespaceStore == nil {
		return nil, &Error{
			Code:       api.ErrorCodeNotFound,
			Message:    "Namespace support not enabled",
			HTTPStatus: 404,
		}
	}

	if request.Body == nil {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "Request body is required",
			HTTPStatus: 400,
		}
	}

	ns, err := a.namespaceStore.GetByName(ctx, request.NamespaceName)
	if err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeNotFound,
			Message:    "Namespace not found",
			HTTPStatus: 404,
		}
	}

	// Prevent renaming the default namespace
	if ns.Name == filens.DefaultNamespaceName {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "Cannot rename the default namespace",
			HTTPStatus: 400,
		}
	}

	if err := a.namespaceStore.Rename(ctx, ns.ID, request.Body.NewName); err != nil {
		if err == filens.ErrNamespaceAlreadyExists {
			return nil, &Error{
				Code:       api.ErrorCodeAlreadyExists,
				Message:    "A namespace with that name already exists",
				HTTPStatus: 400,
			}
		}
		return nil, err
	}

	// Fetch the updated namespace
	ns, err = a.namespaceStore.GetByID(ctx, ns.ID)
	if err != nil {
		return nil, err
	}

	return api.RenameNamespace200JSONResponse(toNamespaceInfo(ns)), nil
}

// toNamespaceInfo converts a filens.Namespace to an api.NamespaceInfo.
func toNamespaceInfo(ns *filens.Namespace) api.NamespaceInfo {
	return api.NamespaceInfo{
		Id:          ns.ID,
		Name:        ns.Name,
		DisplayName: ptrOf(ns.DisplayName),
		Description: ptrOf(ns.Description),
		CreatedAt:   ns.CreatedAt,
		CreatedBy:   ptrOf(ns.CreatedBy),
		UpdatedAt:   ptrOf(ns.UpdatedAt),
	}
}
