// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	openapi "github.com/dagucloud/dagu/api/v1"
	"github.com/stretchr/testify/require"
)

func TestControllerDetailExposesArtifactDirectoryAndAvailability(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	api, svc, _ := newControllerMemoryAPI(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", "trigger:\n  type: manual\ngoal: Complete the assigned software work\nworkflows:\n  names:\n    - build-app\n"))

	detailResp, err := api.GetController(ctx, openapi.GetControllerRequestObject{Name: "software_dev"})
	require.NoError(t, err)
	detailOK, ok := detailResp.(openapi.GetController200JSONResponse)
	require.True(t, ok)
	require.Equal(t, filepath.Join(api.config.Paths.DataDir, "controller", "artifacts", "software_dev"), *detailOK.ArtifactDir)
	require.NotNil(t, detailOK.ArtifactsAvailable)
	require.False(t, *detailOK.ArtifactsAvailable)
	require.DirExists(t, *detailOK.ArtifactDir)

	require.NoError(t, os.WriteFile(filepath.Join(*detailOK.ArtifactDir, "report.md"), []byte("# report"), 0o600))

	detailResp, err = api.GetController(ctx, openapi.GetControllerRequestObject{Name: "software_dev"})
	require.NoError(t, err)
	detailOK, ok = detailResp.(openapi.GetController200JSONResponse)
	require.True(t, ok)
	require.NotNil(t, detailOK.ArtifactsAvailable)
	require.True(t, *detailOK.ArtifactsAvailable)
}

func TestControllerArtifactEndpoints(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	api, svc, _ := newControllerMemoryAPI(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", "trigger:\n  type: manual\ngoal: Complete the assigned software work\nworkflows:\n  names:\n    - build-app\n"))

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(detail.ArtifactDir, "report.md"), []byte("# heading\nhello"), 0o600))

	recursive := openapi.ArtifactRecursive(true)
	listResp, err := api.GetControllerArtifacts(ctx, openapi.GetControllerArtifactsRequestObject{
		Name: "software_dev",
		Params: openapi.GetControllerArtifactsParams{
			Recursive: &recursive,
		},
	})
	require.NoError(t, err)
	listOK, ok := listResp.(openapi.GetControllerArtifacts200JSONResponse)
	require.True(t, ok)
	require.Len(t, listOK.Items, 1)
	require.Equal(t, "report.md", listOK.Items[0].Name)

	previewResp, err := api.GetControllerArtifactPreview(ctx, openapi.GetControllerArtifactPreviewRequestObject{
		Name: "software_dev",
		Params: openapi.GetControllerArtifactPreviewParams{
			Path: openapi.ArtifactPath("report.md"),
		},
	})
	require.NoError(t, err)
	previewOK, ok := previewResp.(openapi.GetControllerArtifactPreview200JSONResponse)
	require.True(t, ok)
	require.Equal(t, openapi.ArtifactPreviewKindMarkdown, previewOK.Kind)
	require.NotNil(t, previewOK.Content)
	require.Contains(t, *previewOK.Content, "# heading")

	downloadResp, err := api.DownloadControllerArtifact(ctx, openapi.DownloadControllerArtifactRequestObject{
		Name: "software_dev",
		Params: openapi.DownloadControllerArtifactParams{
			Path: openapi.ArtifactPath("report.md"),
		},
	})
	require.NoError(t, err)
	downloadOK, ok := downloadResp.(openapi.DownloadControllerArtifact200ApplicationoctetStreamResponse)
	require.True(t, ok)
	content, err := io.ReadAll(downloadOK.Body)
	require.NoError(t, err)
	require.Equal(t, "# heading\nhello", string(content))
}
