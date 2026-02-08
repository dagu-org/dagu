package api

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/service/audit"
)

// ListNamespaces returns namespaces accessible to the current user.
// Admins see all namespaces; other users see only those they have access to.
func (a *API) ListNamespaces(ctx context.Context, _ api.ListNamespacesRequestObject) (api.ListNamespacesResponseObject, error) {
	if a.namespaceStore == nil {
		return nil, &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "namespace store not configured",
		}
	}

	// When auth is enabled, require an authenticated user.
	if a.authService != nil {
		if _, ok := auth.UserFromContext(ctx); !ok {
			return nil, errAuthRequired
		}
	}

	namespaces, err := a.listAccessibleNamespaces(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]api.Namespace, len(namespaces))
	for i, ns := range namespaces {
		result[i] = toAPINamespace(ns)
	}

	a.logAuditEntry(ctx, audit.CategoryNamespace, "namespace_list", nil)

	return api.ListNamespaces200JSONResponse{
		Namespaces: result,
	}, nil
}

// GetNamespace returns details of a specific namespace.
// Requires the user to have access to the namespace (any valid role).
func (a *API) GetNamespace(ctx context.Context, request api.GetNamespaceRequestObject) (api.GetNamespaceResponseObject, error) {
	if err := a.requireNamespaceAccess(ctx, request.NamespaceName); err != nil {
		return nil, err
	}

	if a.namespaceStore == nil {
		return nil, &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "namespace store not configured",
		}
	}

	ns, err := a.namespaceStore.Get(ctx, request.NamespaceName)
	if err != nil {
		if errors.Is(err, exec.ErrNamespaceNotFound) {
			return nil, &Error{
				HTTPStatus: http.StatusNotFound,
				Code:       api.ErrorCodeNotFound,
				Message:    "Namespace not found",
			}
		}
		return nil, err
	}

	a.logAuditEntry(ctx, audit.CategoryNamespace, "namespace_get", map[string]any{
		"namespace": ns.Name,
	})

	return api.GetNamespace200JSONResponse{
		Namespace: toAPINamespace(ns),
	}, nil
}

// CreateNamespace creates a new namespace. Requires admin role.
func (a *API) CreateNamespace(ctx context.Context, request api.CreateNamespaceRequestObject) (api.CreateNamespaceResponseObject, error) {
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if a.namespaceStore == nil {
		return nil, &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "namespace store not configured",
		}
	}

	if request.Body == nil {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeBadRequest,
			Message:    "Invalid request body",
		}
	}

	opts := exec.CreateNamespaceOptions{
		Name:        request.Body.Name,
		Description: valueOf(request.Body.Description),
	}
	if request.Body.Defaults != nil {
		opts.Defaults = exec.NamespaceDefaults{
			Queue:      valueOf(request.Body.Defaults.Queue),
			WorkingDir: valueOf(request.Body.Defaults.WorkingDir),
		}
	}
	if request.Body.GitSync != nil {
		opts.GitSync = exec.NamespaceGitSync{
			RemoteURL:        valueOf(request.Body.GitSync.RemoteURL),
			Branch:           valueOf(request.Body.GitSync.Branch),
			SSHKeyRef:        valueOf(request.Body.GitSync.SshKeyRef),
			Path:             valueOf(request.Body.GitSync.Path),
			AutoSyncInterval: valueOf(request.Body.GitSync.AutoSyncInterval),
		}
	}

	ns, err := a.namespaceStore.Create(ctx, opts)
	if err != nil {
		return nil, err
	}

	a.logAuditEntry(ctx, audit.CategoryNamespace, "namespace_create", map[string]any{
		"namespace": ns.Name,
		"id":        ns.ID,
	})

	return api.CreateNamespace201JSONResponse{
		Namespace: toAPINamespace(ns),
	}, nil
}

// UpdateNamespace updates a namespace's settings. Requires admin role in the namespace.
func (a *API) UpdateNamespace(ctx context.Context, request api.UpdateNamespaceRequestObject) (api.UpdateNamespaceResponseObject, error) {
	if err := a.requireNamespaceAdmin(ctx, request.NamespaceName); err != nil {
		return nil, err
	}

	if a.namespaceStore == nil {
		return nil, &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "namespace store not configured",
		}
	}

	if request.Body == nil {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeBadRequest,
			Message:    "Invalid request body",
		}
	}

	opts := exec.UpdateNamespaceOptions{
		Description: request.Body.Description,
	}
	if request.Body.Defaults != nil {
		opts.Defaults = &exec.NamespaceDefaults{
			Queue:      valueOf(request.Body.Defaults.Queue),
			WorkingDir: valueOf(request.Body.Defaults.WorkingDir),
		}
	}
	if request.Body.GitSync != nil {
		opts.GitSync = &exec.NamespaceGitSync{
			RemoteURL:        valueOf(request.Body.GitSync.RemoteURL),
			Branch:           valueOf(request.Body.GitSync.Branch),
			SSHKeyRef:        valueOf(request.Body.GitSync.SshKeyRef),
			Path:             valueOf(request.Body.GitSync.Path),
			AutoSyncInterval: valueOf(request.Body.GitSync.AutoSyncInterval),
		}
	}
	if request.Body.BaseConfig != nil {
		dag, err := spec.LoadYAML(ctx, []byte(*request.Body.BaseConfig), spec.WithoutEval())
		if err != nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "Invalid base config YAML: " + err.Error(),
			}
		}
		opts.BaseConfig = dag
		opts.BaseConfigYAML = request.Body.BaseConfig
	}

	ns, err := a.namespaceStore.Update(ctx, request.NamespaceName, opts)
	if err != nil {
		return nil, err
	}

	a.logAuditEntry(ctx, audit.CategoryNamespace, "namespace_update", map[string]any{
		"namespace": ns.Name,
	})

	return api.UpdateNamespace200JSONResponse{
		Namespace: toAPINamespace(ns),
	}, nil
}

// DeleteNamespace deletes a namespace. Requires admin role. Namespace must be empty.
func (a *API) DeleteNamespace(ctx context.Context, request api.DeleteNamespaceRequestObject) (api.DeleteNamespaceResponseObject, error) {
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if a.namespaceStore == nil {
		return nil, &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "namespace store not configured",
		}
	}

	name := request.NamespaceName

	// Prevent deletion of the default namespace.
	if name == "default" {
		return nil, &Error{
			HTTPStatus: http.StatusForbidden,
			Code:       api.ErrorCodeForbidden,
			Message:    "Cannot delete the default namespace",
		}
	}

	// Get namespace to resolve the ID for DAG directory check.
	ns, err := a.namespaceStore.Get(ctx, name)
	if err != nil {
		return nil, err
	}

	// Check if the namespace contains DAGs.
	dagDir := filepath.Join(a.config.Paths.DAGsDir, ns.ID)
	if hasDAGs, checkErr := exec.NamespaceHasDAGs(dagDir); checkErr != nil {
		return nil, &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "Failed to check namespace DAGs: " + checkErr.Error(),
		}
	} else if hasDAGs {
		return nil, &Error{
			HTTPStatus: http.StatusConflict,
			Code:       api.ErrorCodeAlreadyExists,
			Message:    "Namespace contains DAGs; remove all DAGs before deleting",
		}
	}

	if err := a.namespaceStore.Delete(ctx, name); err != nil {
		return nil, err
	}

	a.logAuditEntry(ctx, audit.CategoryNamespace, "namespace_delete", map[string]any{
		"namespace": name,
	})

	return api.DeleteNamespace204Response{}, nil
}

// toAPINamespace converts an exec.Namespace to the API representation.
func toAPINamespace(ns *exec.Namespace) api.Namespace {
	result := api.Namespace{
		Name:        ns.Name,
		Id:          ns.ID,
		CreatedAt:   ns.CreatedAt,
		Description: ptrOf(ns.Description),
	}

	if ns.BaseConfigYAML != "" {
		result.BaseConfig = ptrOf(ns.BaseConfigYAML)
	}

	if ns.Defaults.Queue != "" || ns.Defaults.WorkingDir != "" {
		result.Defaults = &api.NamespaceDefaults{
			Queue:      ptrOf(ns.Defaults.Queue),
			WorkingDir: ptrOf(ns.Defaults.WorkingDir),
		}
	}

	if ns.GitSync.RemoteURL != "" || ns.GitSync.Branch != "" {
		result.GitSync = &api.NamespaceGitSync{
			RemoteURL:        ptrOf(ns.GitSync.RemoteURL),
			Branch:           ptrOf(ns.GitSync.Branch),
			SshKeyRef:        ptrOf(ns.GitSync.SSHKeyRef),
			Path:             ptrOf(ns.GitSync.Path),
			AutoSyncInterval: ptrOf(ns.GitSync.AutoSyncInterval),
		}
	}

	return result
}
