package local

import (
	"errors"
	"fmt"
	"github.com/dagu-dev/dagu/internal/persistence/filecache"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/grep"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/util"
)

type dagStoreImpl struct {
	dir       string
	metaCache *filecache.Cache[*dag.DAG]
}

func NewDAGStore(dir string) persistence.DAGStore {
	ds := &dagStoreImpl{
		dir:       dir,
		metaCache: filecache.New[*dag.DAG](0, time.Hour*24),
	}
	ds.metaCache.StartEviction()
	return ds
}

var (
	errInvalidName           = errors.New("invalid name")
	errFailedToReadDAGFile   = errors.New("failed to read DAG file")
	errDOGFileNotExist       = errors.New("the DAG file does not exist")
	errFailedToUpdateDAGFile = errors.New("failed to update DAG file")
	errFailedToCreateDAGFile = errors.New("failed to create DAG file")
	errFailedToCreateDAGsDir = errors.New("failed to create DAGs directory")
	errFailedToDeleteDAGFile = errors.New("failed to delete DAG file")
	errDAGFileAlreadyExists  = errors.New("the DAG file already exists")
	errInvalidNewName        = errors.New("invalid new name")
	errInvalidOldName        = errors.New("invalid old name")
)

func (d *dagStoreImpl) GetMetadata(name string) (*dag.DAG, error) {
	loc, err := d.fileLocation(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errInvalidName, name)
	}
	return d.metaCache.LoadLatest(loc, func() (*dag.DAG, error) {
		return dag.LoadMetadata(loc)
	})
}

func (d *dagStoreImpl) GetDetails(name string) (*dag.DAG, error) {
	loc, err := d.fileLocation(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errInvalidName, name)
	}
	dat, err := dag.LoadWithoutEval(loc)
	if err != nil {
		return nil, err
	}
	return dat, nil
}

func (d *dagStoreImpl) GetSpec(name string) (string, error) {
	loc, err := d.fileLocation(name)
	if err != nil {
		return "", fmt.Errorf("%w: %s", errInvalidName, name)
	}
	dat, err := os.ReadFile(loc)
	if err != nil {
		return "", fmt.Errorf("%w: %s", errFailedToReadDAGFile, err)
	}
	return string(dat), nil
}

func (d *dagStoreImpl) UpdateSpec(name string, spec []byte) error {
	// validation
	_, err := dag.LoadYAML(spec)
	if err != nil {
		return err
	}
	loc, err := d.fileLocation(name)
	if err != nil {
		return fmt.Errorf("%w: %s", errInvalidName, name)
	}
	if !exists(loc) {
		return fmt.Errorf("%w: %s", errDOGFileNotExist, loc)
	}
	err = os.WriteFile(loc, spec, 0755)
	if err != nil {
		return fmt.Errorf("%w: %s", errFailedToUpdateDAGFile, err)
	}
	d.metaCache.Invalidate(loc)
	return nil
}

func (d *dagStoreImpl) Create(name string, spec []byte) (string, error) {
	if err := d.ensureDirExist(); err != nil {
		return "", fmt.Errorf("%w: %s", errFailedToCreateDAGsDir, d.dir)
	}
	loc, err := d.fileLocation(name)
	if err != nil {
		return "", fmt.Errorf("%w: %s", errFailedToCreateDAGFile, name)
	}
	if exists(loc) {
		return "", fmt.Errorf("%w: %s", errDAGFileAlreadyExists, loc)
	}
	return name, os.WriteFile(loc, spec, 0644)
}

func (d *dagStoreImpl) Delete(name string) error {
	loc, err := d.fileLocation(name)
	if err != nil {
		return fmt.Errorf("%w: %s", errFailedToCreateDAGFile, name)
	}
	err = os.Remove(loc)
	if err != nil {
		return fmt.Errorf("%w: %s", errFailedToDeleteDAGFile, err)
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
	loc := path.Join(d.dir, name)
	return d.normalizeFilename(loc)
}

func (d *dagStoreImpl) normalizeFilename(file string) (string, error) {
	a := strings.TrimSuffix(file, ".yaml")
	a = strings.TrimSuffix(a, ".yml")
	return fmt.Sprintf("%s.yaml", a), nil
}

func (d *dagStoreImpl) ensureDirExist() error {
	if !exists(d.dir) {
		if err := os.MkdirAll(d.dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func (d *dagStoreImpl) List() (ret []*dag.DAG, errs []string, err error) {
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
			dat, err := d.GetMetadata(fi.Name())
			if err == nil {
				ret = append(ret, dat)
			} else {
				errs = append(errs, fmt.Sprintf("reading %s failed: %s", fi.Name(), err))
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

func (d *dagStoreImpl) Grep(pattern string) (ret []*persistence.GrepResult, errs []string, err error) {
	if err = d.ensureDirExist(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to create DAGs directory %s", d.dir))
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
		if util.MatchExtension(fi.Name(), dag.EXTENSIONS) {
			file := filepath.Join(d.dir, fi.Name())
			dat, err := os.ReadFile(file)
			if err != nil {
				util.LogErr("read DAG file", err)
				continue
			}
			m, err := grep.Grep(dat, fmt.Sprintf("(?i)%s", pattern), opts)
			if err != nil {
				errs = append(errs, fmt.Sprintf("grep %s failed: %s", fi.Name(), err))
				continue
			}
			d, err := dag.LoadMetadata(file)
			if err != nil {
				errs = append(errs, fmt.Sprintf("check %s failed: %s", fi.Name(), err))
				continue
			}
			ret = append(ret, &persistence.GrepResult{
				Name:    strings.TrimSuffix(fi.Name(), path.Ext(fi.Name())),
				DAG:     d,
				Matches: m,
			})
		}
	}
	return ret, errs, nil
}

func (d *dagStoreImpl) Load(name string) (*dag.DAG, error) {
	// TODO implement me
	panic("implement me")
}

func (d *dagStoreImpl) Rename(oldDAGPath, newDAGPath string) error {
	oldLoc, err := d.fileLocation(oldDAGPath)
	if err != nil {
		return fmt.Errorf("%w: %s", errInvalidOldName, oldDAGPath)
	}
	newLoc, err := d.fileLocation(newDAGPath)
	if err != nil {
		return fmt.Errorf("%w: %s", errInvalidNewName, newDAGPath)
	}
	return os.Rename(oldLoc, newLoc)
}

func (d *dagStoreImpl) Find(name string) (*dag.DAG, error) {
	file, err := d.resolve(name)
	if err != nil {
		return nil, err
	}
	return dag.LoadWithoutEval(file)
}

func (d *dagStoreImpl) resolve(name string) (string, error) {
	// check if the name is a file path
	if strings.Contains(name, string(filepath.Separator)) {
		if !util.FileExists(name) {
			return "", fmt.Errorf("workflow %s not found", name)
		}
		return name, nil
	}

	// check if the name is a file path
	if strings.Contains(name, string(filepath.Separator)) {
		foundPath, err := find(name)
		if err != nil {
			return "", fmt.Errorf("workflow %s not found", name)
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
		for _, ext := range dag.EXTENSIONS {
			if util.FileExists(name + ext) {
				return filepath.Abs(name + ext)
			}
		}
	} else if util.FileExists(name) {
		// the name has an extension
		return filepath.Abs(name)
	}
	return "", fmt.Errorf("sub workflow %s not found", name)
}
