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
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
	"github.com/dagu-org/dagu/internal/persistence/grep"
	"github.com/dagu-org/dagu/internal/util"
)

type dagStoreImpl struct {
	dir       string
	metaCache *filecache.Cache[*digraph.DAG]
}

type NewDAGStoreArgs struct {
	Dir string
}

func NewDAGStore(args *NewDAGStoreArgs) persistence.DAGStore {
	dagStore := &dagStoreImpl{
		dir:       args.Dir,
		metaCache: filecache.New[*digraph.DAG](0, time.Hour*24),
	}
	dagStore.metaCache.StartEviction()
	return dagStore
}

func (d *dagStoreImpl) GetMetadata(ctx context.Context, name string) (*digraph.DAG, error) {
	loc, err := d.fileLocation(name)
	if err != nil {
		return nil, err
	}
	return d.metaCache.LoadLatest(loc, func() (*digraph.DAG, error) {
		return digraph.LoadMetadata(ctx, loc)
	})
}

func (d *dagStoreImpl) GetDetails(ctx context.Context, name string) (*digraph.DAG, error) {
	loc, err := d.fileLocation(name)
	if err != nil {
		return nil, err
	}
	dat, err := digraph.LoadWithoutEval(ctx, loc)
	if err != nil {
		return nil, err
	}
	return dat, nil
}

func (d *dagStoreImpl) GetSpec(_ context.Context, name string) (string, error) {
	loc, err := d.fileLocation(name)
	if err != nil {
		return "", err
	}
	dat, err := os.ReadFile(loc)
	if err != nil {
		return "", err
	}
	return string(dat), nil
}

// TODO: use 0600 // nolint: gosec
const defaultPerm os.FileMode = 0744

var errDOGFileNotExist = errors.New("the DAG file does not exist")

func (d *dagStoreImpl) UpdateSpec(ctx context.Context, name string, spec []byte) error {
	// validation
	_, err := digraph.LoadYAML(ctx, spec)
	if err != nil {
		return err
	}
	loc, err := d.fileLocation(name)
	if err != nil {
		return err
	}
	if !exists(loc) {
		return fmt.Errorf("%w: %s", errDOGFileNotExist, loc)
	}
	err = os.WriteFile(loc, spec, defaultPerm)
	if err != nil {
		return err
	}
	d.metaCache.Invalidate(loc)
	return nil
}

var errDAGFileAlreadyExists = errors.New("the DAG file already exists")

func (d *dagStoreImpl) Create(_ context.Context, name string, spec []byte) (string, error) {
	if err := d.ensureDirExist(); err != nil {
		return "", err
	}
	loc, err := d.fileLocation(name)
	if err != nil {
		return "", err
	}
	if exists(loc) {
		return "", fmt.Errorf("%w: %s", errDAGFileAlreadyExists, loc)
	}
	// nolint: gosec
	return name, os.WriteFile(loc, spec, 0644)
}

func (d *dagStoreImpl) Delete(_ context.Context, name string) error {
	loc, err := d.fileLocation(name)
	if err != nil {
		return err
	}
	err = os.Remove(loc)
	if err != nil {
		return err
	}
	d.metaCache.Invalidate(loc)
	return nil
}

func exists(file string) bool {
	_, err := os.Stat(file)
	return !os.IsNotExist(err)
}

func (d *dagStoreImpl) fileLocation(name string) (string, error) {
	if strings.Contains(name, "/") {
		// this is for backward compatibility
		return name, nil
	}
	return fileutil.AddYAMLExtension(path.Join(d.dir, name)), nil
}

func (d *dagStoreImpl) ensureDirExist() error {
	if !exists(d.dir) {
		if err := os.MkdirAll(d.dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func (d *dagStoreImpl) searchName(fileName string, searchText *string) bool {
	if searchText == nil {
		return true
	}
	fileName = strings.TrimSuffix(fileName, path.Ext(fileName))
	ret := strings.Contains(strings.ToLower(fileName), strings.ToLower(*searchText))
	return ret
}

func (d *dagStoreImpl) searchTags(tags []string, searchTag *string) bool {
	if searchTag == nil {
		return true
	}

	for _, tag := range tags {
		if tag == *searchTag {
			return true
		}
	}

	return false
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
		dagList    = make([]*digraph.DAG, 0)
		errList    = make([]string, 0)
		count      int
		currentDag *digraph.DAG
	)

	if err := filepath.WalkDir(d.dir, func(_ string, dir fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if dir.IsDir() || !checkExtension(dir.Name()) {
			return nil
		}

		if currentDag, err = d.GetMetadata(ctx, dir.Name()); err != nil {
			errList = append(errList, fmt.Sprintf("reading %s failed: %s", dir.Name(), err))
		}

		if !d.searchName(dir.Name(), params.Name) || currentDag == nil || !d.searchTags(currentDag.Tags, params.Tag) {
			return nil
		}

		count++
		if count > (params.Page-1)*params.Limit && len(dagList) < params.Limit {
			dagList = append(dagList, currentDag)
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
	fis, err := os.ReadDir(d.dir)
	if err != nil {
		errs = append(errs, err.Error())
		return
	}
	for _, fi := range fis {
		if checkExtension(fi.Name()) {
			dat, err := d.GetMetadata(ctx, fi.Name())
			if err == nil {
				ret = append(ret, dat)
			} else {
				errs = append(errs, fmt.Sprintf(
					"reading %s failed: %s", fi.Name(), err),
				)
			}
		}
	}
	return ret, errs, nil
}

var extensions = []string{".yaml", ".yml"}

func checkExtension(file string) bool {
	ext := filepath.Ext(file)
	for _, e := range extensions {
		if e == ext {
			return true
		}
	}
	return false
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

	fis, err := os.ReadDir(d.dir)
	opts := &grep.Options{
		IsRegexp: true,
		Before:   2,
		After:    2,
	}

	util.LogErr("read DAGs directory", err)
	for _, fi := range fis {
		if fileutil.IsYAMLFile(fi.Name()) {
			file := filepath.Join(d.dir, fi.Name())
			dat, err := os.ReadFile(file)
			if err != nil {
				util.LogErr("read DAG file", err)
				continue
			}
			m, err := grep.Grep(dat, fmt.Sprintf("(?i)%s", pattern), opts)
			if err != nil {
				errs = append(
					errs, fmt.Sprintf("grep %s failed: %s", fi.Name(), err),
				)
				continue
			}
			dag, err := digraph.LoadMetadata(ctx, file)
			if err != nil {
				errs = append(
					errs, fmt.Sprintf("check %s failed: %s", fi.Name(), err),
				)
				continue
			}
			ret = append(ret, &persistence.GrepResult{
				Name:    strings.TrimSuffix(fi.Name(), path.Ext(fi.Name())),
				DAG:     dag,
				Matches: m,
			})
		}
	}
	return ret, errs, nil
}

func (d *dagStoreImpl) Rename(_ context.Context, oldID, newID string) error {
	oldLoc, err := d.fileLocation(oldID)
	if err != nil {
		return err
	}
	newLoc, err := d.fileLocation(newID)
	if err != nil {
		return err
	}
	return os.Rename(oldLoc, newLoc)
}

func (d *dagStoreImpl) Find(ctx context.Context, name string) (*digraph.DAG, error) {
	file, err := d.resolve(name)
	if err != nil {
		return nil, err
	}
	return digraph.LoadWithoutEval(ctx, file)
}

func (d *dagStoreImpl) resolve(name string) (string, error) {
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

		if dir.IsDir() || !checkExtension(dir.Name()) {
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
