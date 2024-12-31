// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package local

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
	"github.com/dagu-org/dagu/internal/persistence/grep"
)

var _ persistence.DAGStore = (*dagStoreImpl)(nil)
var _ digraph.Finder = (*dagStoreImpl)(nil)

type dagStoreImpl struct {
	dir       string
	metaCache *filecache.Cache[*digraph.DAG]
}

const defaultCacheTTL = time.Hour * 24

func NewDAGStore(dir string) persistence.DAGStore {
	metaCache := filecache.New[*digraph.DAG](0, defaultCacheTTL)
	metaCache.StartEviction()

	return &dagStoreImpl{
		dir:       dir,
		metaCache: metaCache,
	}
}

func (d *dagStoreImpl) GetMetadata(ctx context.Context, name string) (*digraph.DAG, error) {
	filePath := resolveFilePath(d.dir, name)
	return d.metaCache.LoadLatest(filePath, func() (*digraph.DAG, error) {
		return digraph.LoadMetadata(ctx, filePath)
	})
}

func (d *dagStoreImpl) GetDetails(ctx context.Context, name string) (*digraph.DAG, error) {
	filePath := resolveFilePath(d.dir, name)
	dat, err := digraph.LoadWithoutEval(ctx, filePath)
	if err != nil {
		return nil, err
	}
	return dat, nil
}

func (d *dagStoreImpl) GetSpec(_ context.Context, name string) (string, error) {
	filePath := resolveFilePath(d.dir, name)
	dat, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(dat), nil
}

// TODO: use 0600 // nolint: gosec
const defaultPerm os.FileMode = 0744

var errDOGFileNotExist = errors.New("the DAG file does not exist")

func (d *dagStoreImpl) UpdateSpec(ctx context.Context, name string, spec []byte) error {
	// Load the DAG to validate the spec.
	_, err := digraph.LoadYAML(ctx, spec)
	if err != nil {
		return err
	}
	filePath := resolveFilePath(d.dir, name)
	if !exists(filePath) {
		return fmt.Errorf("%w: %s", errDOGFileNotExist, filePath)
	}
	if err := os.WriteFile(filePath, spec, defaultPerm); err != nil {
		return err
	}
	d.metaCache.Invalidate(filePath)
	return nil
}

var errDAGFileAlreadyExists = errors.New("the DAG file already exists")

func (d *dagStoreImpl) Create(_ context.Context, name string, spec []byte) (string, error) {
	if err := d.ensureDirExist(); err != nil {
		return "", err
	}
	filePath := resolveFilePath(d.dir, name)
	if exists(filePath) {
		return "", fmt.Errorf("%w: %s", errDAGFileAlreadyExists, filePath)
	}
	// nolint: gosec
	return name, os.WriteFile(filePath, spec, 0644)
}

func (d *dagStoreImpl) Delete(_ context.Context, name string) error {
	filePath := resolveFilePath(d.dir, name)
	if err := os.Remove(filePath); err != nil {
		return err
	}
	d.metaCache.Invalidate(filePath)
	return nil
}

func (d *dagStoreImpl) ensureDirExist() error {
	if !exists(d.dir) {
		if err := os.MkdirAll(d.dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func (d *dagStoreImpl) getTagList(tagSet map[string]struct{}) []string {
	tagList := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tagList = append(tagList, tag)
	}
	return tagList
}

func (d *dagStoreImpl) ListPagination(ctx context.Context, params persistence.DAGListPaginationArgs) (*persistence.DagListPaginationResult, error) {
	var (
		dagList []*digraph.DAG
		errList []string
		count   int
	)

	if err := filepath.WalkDir(d.dir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() || !fileutil.IsYAMLFile(entry.Name()) {
			return nil
		}

		baseName := path.Base(entry.Name())
		dagName := strings.TrimSuffix(baseName, path.Ext(baseName))
		if params.Name != "" && params.Tag == "" {
			// If tag is not provided, check before reading the file to avoid
			// unnecessary file read and parsing.
			if !containsSearchText(dagName, params.Name) {
				// Return early if the name does not match the search text.
				return nil
			}
		}

		// Read the file and parse the DAG.
		parsedDAG, err := d.GetMetadata(ctx, dagName)
		if err != nil {
			errList = append(errList, fmt.Sprintf("reading %s failed: %s", dagName, err))
			return nil
		}

		if params.Name != "" && !containsSearchText(dagName, params.Name) {
			return nil
		}

		if params.Tag != "" && !containsTag(parsedDAG.Tags, params.Tag) {
			return nil
		}

		count++
		if count > (params.Page-1)*params.Limit && len(dagList) < params.Limit {
			dagList = append(dagList, parsedDAG)
		}

		return nil
	}); err != nil {
		return &persistence.DagListPaginationResult{
			DagList:   dagList,
			Count:     count,
			ErrorList: append(errList, err.Error()),
		}, err
	}

	return &persistence.DagListPaginationResult{
		DagList:   dagList,
		Count:     count,
		ErrorList: errList,
	}, nil
}

func (d *dagStoreImpl) List(ctx context.Context) (ret []*digraph.DAG, errs []string, err error) {
	if err = d.ensureDirExist(); err != nil {
		errs = append(errs, err.Error())
		return
	}
	entries, err := os.ReadDir(d.dir)
	if err != nil {
		errs = append(errs, err.Error())
		return
	}
	for _, entry := range entries {
		if fileutil.IsYAMLFile(entry.Name()) {
			dat, err := d.GetMetadata(ctx, entry.Name())
			if err == nil {
				ret = append(ret, dat)
			} else {
				errs = append(errs, fmt.Sprintf(
					"reading %s failed: %s", entry.Name(), err),
				)
			}
		}
	}
	return ret, errs, nil
}

func (d *dagStoreImpl) Grep(ctx context.Context, pattern string) (
	ret []*persistence.GrepResult, errs []string, err error,
) {
	if err = d.ensureDirExist(); err != nil {
		errs = append(
			errs, fmt.Sprintf("failed to create DAGs directory %s", d.dir),
		)
		return
	}

	entries, err := os.ReadDir(d.dir)
	if err != nil {
		logger.Error(ctx, "Failed to read directory", "dir", d.dir, "err", err)
	}

	opts := &grep.Options{
		IsRegexp: true,
		Before:   2,
		After:    2,
	}

	for _, entry := range entries {
		if fileutil.IsYAMLFile(entry.Name()) {
			filePath := filepath.Join(d.dir, entry.Name())
			dat, err := os.ReadFile(filePath)
			if err != nil {
				logger.Error(ctx, "Failed to read DAG file", "file", entry.Name(), "err", err)
				continue
			}
			matches, err := grep.Grep(dat, fmt.Sprintf("(?i)%s", pattern), opts)
			if err != nil {
				errs = append(errs, fmt.Sprintf("grep %s failed: %s", entry.Name(), err))
				continue
			}
			dag, err := digraph.LoadMetadata(ctx, filePath)
			if err != nil {
				errs = append(errs, fmt.Sprintf("check %s failed: %s", entry.Name(), err))
				continue
			}
			ret = append(ret, &persistence.GrepResult{
				Name:    strings.TrimSuffix(entry.Name(), path.Ext(entry.Name())),
				DAG:     dag,
				Matches: matches,
			})
		}
	}
	return ret, errs, nil
}

func (d *dagStoreImpl) Rename(_ context.Context, oldID, newID string) error {
	oldFilePath := resolveFilePath(d.dir, oldID)
	newFilePath := resolveFilePath(d.dir, newID)
	return os.Rename(oldFilePath, newFilePath)
}

func (d *dagStoreImpl) FindByName(ctx context.Context, name string) (*digraph.DAG, error) {
	file, err := d.locateDAG(name)
	if err != nil {
		return nil, err
	}
	return digraph.LoadWithoutEval(ctx, file)
}

func (d *dagStoreImpl) locateDAG(name string) (string, error) {
	// check if the name is a file path
	if strings.Contains(name, string(filepath.Separator)) {
		if !fileutil.FileExists(name) {
			return "", fmt.Errorf("DAG %s not found", name)
		}
		return name, nil
	}

	// check if the name is a file path
	if strings.Contains(name, string(filepath.Separator)) {
		foundPath, err := find(name)
		if err != nil {
			return "", fmt.Errorf("DAG %s not found", name)
		}
		return foundPath, nil
	}

	// find the DAG definition
	for _, dir := range []string{".", d.dir} {
		subWorkflowPath := filepath.Join(dir, name)
		foundPath, err := find(subWorkflowPath)
		if err == nil {
			return foundPath, nil
		}
	}

	// DAG not found
	return "", fmt.Errorf("workflow %s not found", name)
}

// find finds the sub workflow file with the given name.
func find(name string) (string, error) {
	ext := path.Ext(name)
	if ext == "" {
		// try all supported extensions
		for _, ext := range fileutil.ValidYAMLExtensions {
			if fileutil.FileExists(name + ext) {
				return filepath.Abs(name + ext)
			}
		}
	} else if fileutil.FileExists(name) {
		// the name has an extension
		return filepath.Abs(name)
	}
	return "", fmt.Errorf("sub workflow %s not found", name)
}

func (d *dagStoreImpl) TagList(ctx context.Context) ([]string, []string, error) {
	var (
		errList    = make([]string, 0)
		tagSet     = make(map[string]struct{})
		currentDag *digraph.DAG
		err        error
	)

	if err = filepath.WalkDir(d.dir, func(_ string, dir fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if dir.IsDir() || !fileutil.IsYAMLFile(dir.Name()) {
			return nil
		}

		if currentDag, err = d.GetMetadata(ctx, dir.Name()); err != nil {
			errList = append(errList, fmt.Sprintf("reading %s failed: %s", dir.Name(), err))
		}

		if currentDag == nil {
			return nil
		}

		for _, tag := range currentDag.Tags {
			tagSet[tag] = struct{}{}
		}

		return nil
	}); err != nil {
		return nil, append(errList, err.Error()), err
	}

	return d.getTagList(tagSet), errList, nil
}

func containsSearchText(text string, search string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(search))
}

func containsTag(tags []string, searchTag string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, searchTag) {
			return true
		}
	}

	return false
}

func exists(file string) bool {
	_, err := os.Stat(file)
	return !os.IsNotExist(err)
}

func resolveFilePath(dir, name string) string {
	if filepath.IsAbs(name) {
		// If the name is an absolute path, return it as is.
		return name
	}
	filePath := fileutil.EnsureYAMLExtension(path.Join(dir, name))
	return filepath.Clean(filePath)
}
