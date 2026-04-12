// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package coordinator

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/dagucloud/dagu/internal/cmn/fileutil"
	"github.com/dagucloud/dagu/internal/core/exec"
	coordinatorv1 "github.com/dagucloud/dagu/proto/coordinator/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type artifactHandler struct {
	dagRunStore exec.DAGRunStore
	ownerID     string

	writers   map[string]*logWriter
	writersMu sync.Mutex
}

func newArtifactHandler(dagRunStore exec.DAGRunStore, ownerID string) *artifactHandler {
	return &artifactHandler{
		dagRunStore: dagRunStore,
		ownerID:     ownerID,
		writers:     make(map[string]*logWriter),
	}
}

func (h *artifactHandler) handleStream(stream coordinatorv1.CoordinatorService_StreamArtifactsServer) error {
	ctx := stream.Context()
	var chunksReceived uint64
	var bytesWritten uint64

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&coordinatorv1.StreamArtifactsResponse{
				ChunksReceived: chunksReceived,
				BytesWritten:   bytesWritten,
			})
		}
		if err != nil {
			return fmt.Errorf("failed to receive artifact chunk: %w", err)
		}

		chunksReceived++

		if h.ownerID != "" && chunk.OwnerCoordinatorId != h.ownerID {
			return status.Error(codes.FailedPrecondition, "artifact chunk sent to non-owner coordinator")
		}

		if chunk.IsFinal {
			if _, err := h.getOrCreateWriter(ctx, chunk); err != nil {
				return fmt.Errorf("failed to create artifact writer: %w", err)
			}
			h.closeWriter(ctx, chunk)
			continue
		}
		if len(chunk.Data) == 0 {
			continue
		}

		writer, err := h.getOrCreateWriter(ctx, chunk)
		if err != nil {
			return fmt.Errorf("failed to create artifact writer: %w", err)
		}

		n, err := writer.write(chunk.Data)
		if err != nil {
			return fmt.Errorf("failed to write artifact data: %w", err)
		}
		if n > 0 {
			bytesWritten += uint64(n) // #nosec G115 -- n is non-negative from successful Write
		}
	}
}

func (h *artifactHandler) streamKey(chunk *coordinatorv1.ArtifactChunk) string {
	return fmt.Sprintf("%s/%s/%s/%s",
		chunk.DagName,
		chunk.DagRunId,
		chunk.AttemptId,
		chunk.RelativePath,
	)
}

func (h *artifactHandler) getOrCreateWriter(ctx context.Context, chunk *coordinatorv1.ArtifactChunk) (*logWriter, error) {
	key := h.streamKey(chunk)

	h.writersMu.Lock()
	defer h.writersMu.Unlock()

	if w, ok := h.writers[key]; ok {
		return w, nil
	}

	filePath, err := h.artifactFilePath(ctx, chunk)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0o750); err != nil {
		return nil, fmt.Errorf("failed to create artifact directory: %w", err)
	}

	file, err := fileutil.OpenOrCreateFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open artifact file: %w", err)
	}

	w := &logWriter{
		file:   file,
		writer: bufio.NewWriterSize(file, 64*1024),
		path:   filePath,
	}
	h.writers[key] = w
	return w, nil
}

func (h *artifactHandler) artifactFilePath(ctx context.Context, chunk *coordinatorv1.ArtifactChunk) (string, error) {
	archiveDir, err := h.archiveDir(ctx, chunk)
	if err != nil {
		return "", err
	}
	filePath, err := fileutil.ResolvePathWithinBase(archiveDir, chunk.RelativePath)
	if err != nil {
		return "", fmt.Errorf("resolve artifact path %q: %w", chunk.RelativePath, err)
	}
	return filePath, nil
}

func (h *artifactHandler) archiveDir(ctx context.Context, chunk *coordinatorv1.ArtifactChunk) (string, error) {
	var (
		attempt exec.DAGRunAttempt
		err     error
	)
	if chunk.RootDagRunId != "" && chunk.RootDagRunId != chunk.DagRunId {
		attempt, err = h.dagRunStore.FindSubAttempt(ctx, exec.DAGRunRef{
			Name: chunk.RootDagRunName,
			ID:   chunk.RootDagRunId,
		}, chunk.DagRunId)
	} else {
		attempt, err = h.dagRunStore.FindAttempt(ctx, exec.DAGRunRef{
			Name: chunk.DagName,
			ID:   chunk.DagRunId,
		})
	}
	if err != nil {
		return "", fmt.Errorf("find DAG run attempt for artifacts: %w", err)
	}

	runStatus, err := attempt.ReadStatus(ctx)
	if err != nil {
		return "", fmt.Errorf("read DAG run status for artifacts: %w", err)
	}
	if runStatus == nil || runStatus.ArchiveDir == "" {
		return "", fmt.Errorf("artifact directory is not available for dag run %s", chunk.DagRunId)
	}

	return runStatus.ArchiveDir, nil
}

func (h *artifactHandler) closeWriter(ctx context.Context, chunk *coordinatorv1.ArtifactChunk) {
	key := h.streamKey(chunk)

	h.writersMu.Lock()
	defer h.writersMu.Unlock()

	if w, ok := h.writers[key]; ok {
		w.close(ctx)
		delete(h.writers, key)
	}
}

func (h *artifactHandler) Close(ctx context.Context) {
	h.writersMu.Lock()
	defer h.writersMu.Unlock()

	for _, w := range h.writers {
		w.close(ctx)
	}
	h.writers = make(map[string]*logWriter)
}
