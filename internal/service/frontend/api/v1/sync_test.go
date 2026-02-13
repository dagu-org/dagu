package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	apigen "github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/gitsync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSyncService struct {
	publishAllFn func(ctx context.Context, message string, dagIDs []string) (*gitsync.SyncResult, error)
	getStatusFn  func(context.Context) (*gitsync.OverallStatus, error)
}

func (m *mockSyncService) Pull(_ context.Context) (*gitsync.SyncResult, error) { return nil, nil }

func (m *mockSyncService) Publish(_ context.Context, _ string, _ string, _ bool) (*gitsync.SyncResult, error) {
	return nil, nil
}

func (m *mockSyncService) PublishAll(ctx context.Context, message string, dagIDs []string) (*gitsync.SyncResult, error) {
	if m.publishAllFn == nil {
		return nil, nil
	}
	return m.publishAllFn(ctx, message, dagIDs)
}

func (m *mockSyncService) Discard(_ context.Context, _ string) error { return nil }

func (m *mockSyncService) GetStatus(ctx context.Context) (*gitsync.OverallStatus, error) {
	if m.getStatusFn != nil {
		return m.getStatusFn(ctx)
	}
	return nil, nil
}

func (m *mockSyncService) GetDAGStatus(_ context.Context, _ string) (*gitsync.DAGState, error) {
	return nil, nil
}

func (m *mockSyncService) GetDAGDiff(_ context.Context, _ string) (*gitsync.DAGDiff, error) {
	return nil, nil
}

func (m *mockSyncService) GetConfig(_ context.Context) (*gitsync.Config, error) { return nil, nil }

func (m *mockSyncService) UpdateConfig(_ context.Context, _ *gitsync.Config) error { return nil }

func (m *mockSyncService) TestConnection(_ context.Context) (*gitsync.ConnectionResult, error) {
	return nil, nil
}

func newSyncAPIForTest(syncSvc SyncService) *API {
	return &API{
		config: &config.Config{
			Server: config.Server{
				Permissions: map[config.Permission]bool{
					config.PermissionWriteDAGs: true,
				},
			},
		},
		syncService: syncSvc,
	}
}

func TestSyncPublishAll_Validation(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 for nil request body", func(t *testing.T) {
		t.Parallel()

		a := newSyncAPIForTest(&mockSyncService{})
		_, err := a.SyncPublishAll(context.Background(), apigen.SyncPublishAllRequestObject{})
		require.Error(t, err)

		var apiErr *Error
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.HTTPStatus)
	})

	t.Run("returns 400 for empty dagIds", func(t *testing.T) {
		t.Parallel()

		a := newSyncAPIForTest(&mockSyncService{})
		_, err := a.SyncPublishAll(context.Background(), apigen.SyncPublishAllRequestObject{
			Body: &apigen.SyncPublishAllRequest{
				DagIds: ptrOf([]string{}),
			},
		})
		require.Error(t, err)

		var apiErr *Error
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.HTTPStatus)
		assert.Contains(t, apiErr.Message, "No modified or untracked")
	})

	t.Run("defaults missing dagIds to publishable DAGs from status", func(t *testing.T) {
		t.Parallel()

		var gotIDs []string
		a := newSyncAPIForTest(&mockSyncService{
			getStatusFn: func(_ context.Context) (*gitsync.OverallStatus, error) {
				now := time.Now()
				return &gitsync.OverallStatus{
					DAGs: map[string]*gitsync.DAGState{
						"zeta":    {Status: gitsync.StatusModified, ModifiedAt: &now},
						"alpha":   {Status: gitsync.StatusUntracked, ModifiedAt: &now},
						"ignored": {Status: gitsync.StatusSynced, LastSyncedAt: &now},
					},
				}, nil
			},
			publishAllFn: func(_ context.Context, _ string, dagIDs []string) (*gitsync.SyncResult, error) {
				gotIDs = dagIDs
				return &gitsync.SyncResult{
					Success:   true,
					Synced:    dagIDs,
					Timestamp: time.Now(),
				}, nil
			},
		})

		resp, err := a.SyncPublishAll(context.Background(), apigen.SyncPublishAllRequestObject{
			Body: &apigen.SyncPublishAllRequest{
				Message: ptrOf("publish all publishable"),
			},
		})
		require.NoError(t, err)
		_, ok := resp.(apigen.SyncPublishAll200JSONResponse)
		require.True(t, ok)
		assert.Equal(t, []string{"alpha", "zeta"}, gotIDs)
	})

	t.Run("maps validation error from service to 400", func(t *testing.T) {
		t.Parallel()

		a := newSyncAPIForTest(&mockSyncService{
			publishAllFn: func(_ context.Context, _ string, _ []string) (*gitsync.SyncResult, error) {
				return nil, &gitsync.ValidationError{
					Field:   "dagIds",
					Message: "DAG \"missing\" is not tracked by git sync",
				}
			},
		})

		_, err := a.SyncPublishAll(context.Background(), apigen.SyncPublishAllRequestObject{
			Body: &apigen.SyncPublishAllRequest{
				DagIds: ptrOf([]string{"missing"}),
			},
		})
		require.Error(t, err)

		var apiErr *Error
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.HTTPStatus)
		assert.Contains(t, apiErr.Message, "not tracked")
	})

	t.Run("maps invalid DAG ID error from service to 400", func(t *testing.T) {
		t.Parallel()

		a := newSyncAPIForTest(&mockSyncService{
			publishAllFn: func(_ context.Context, _ string, _ []string) (*gitsync.SyncResult, error) {
				return nil, &gitsync.InvalidDAGIDError{
					DAGID:  "../etc/passwd",
					Reason: "path traversal is not allowed",
				}
			},
		})

		_, err := a.SyncPublishAll(context.Background(), apigen.SyncPublishAllRequestObject{
			Body: &apigen.SyncPublishAllRequest{
				DagIds: ptrOf([]string{"../etc/passwd"}),
			},
		})
		require.Error(t, err)

		var apiErr *Error
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.HTTPStatus)
		assert.Contains(t, apiErr.Message, "invalid DAG ID")
	})

	t.Run("passes dag IDs to service and returns 200", func(t *testing.T) {
		t.Parallel()

		var gotMessage string
		var gotIDs []string
		a := newSyncAPIForTest(&mockSyncService{
			publishAllFn: func(_ context.Context, message string, dagIDs []string) (*gitsync.SyncResult, error) {
				gotMessage = message
				gotIDs = dagIDs
				return &gitsync.SyncResult{
					Success:   true,
					Synced:    []string{"a"},
					Timestamp: time.Now(),
				}, nil
			},
		})

		resp, err := a.SyncPublishAll(context.Background(), apigen.SyncPublishAllRequestObject{
			Body: &apigen.SyncPublishAllRequest{
				Message: ptrOf("publish selected"),
				DagIds:  ptrOf([]string{"b", "a"}),
			},
		})
		require.NoError(t, err)

		_, ok := resp.(apigen.SyncPublishAll200JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "publish selected", gotMessage)
		assert.Equal(t, []string{"a", "b"}, gotIDs)
	})
}
