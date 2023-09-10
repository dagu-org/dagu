package local

import (
	"fmt"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/grep"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/utils"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type dagStoreImpl struct {
	dir string
}

func NewDAGStore(dir string) persistence.DAGStore {
	return &dagStoreImpl{dir: dir}
}

func (d *dagStoreImpl) GetMetadata(name string) (*dag.DAG, error) {
	loc, err := d.fileLocation(name)
	if err != nil {
		return nil, fmt.Errorf("invalid name: %s", name)
	}
	cl := dag.Loader{}
	dat, err := cl.LoadMetadata(loc)
	if err != nil {
		return nil, err
	}
	return dat, nil
}

func (d *dagStoreImpl) GetDetails(name string) (*dag.DAG, error) {
	loc, err := d.fileLocation(name)
	if err != nil {
		return nil, fmt.Errorf("invalid name: %s", name)
	}
	cl := dag.Loader{}
	dat, err := cl.LoadWithoutEval(loc)
	if err != nil {
		return nil, err
	}
	return dat, nil
}

func (d *dagStoreImpl) GetSpec(name string) (string, error) {
	loc, err := d.fileLocation(name)
	if err != nil {
		return "", fmt.Errorf("invalid name: %s", name)
	}
	dat, err := os.ReadFile(loc)
	if err != nil {
		return "", fmt.Errorf("failed to read DAG file: %s", err)
	}
	return string(dat), nil
}

func (d *dagStoreImpl) UpdateSpec(name string, spec []byte) error {
	// validation
	cl := dag.Loader{}
	_, err := cl.LoadData(spec)
	if err != nil {
		return err
	}
	loc, err := d.fileLocation(name)
	if err != nil {
		return fmt.Errorf("invalid name: %s", name)
	}
	if !exists(loc) {
		return fmt.Errorf("the DAG file %s does not exist", loc)
	}
	err = os.WriteFile(loc, spec, 0755)
	if err != nil {
		return fmt.Errorf("failed to update DAG file: %s", err)
	}
	return nil
}

func (d *dagStoreImpl) Create(name string, spec []byte) (string, error) {
	if err := d.ensureDirExist(); err != nil {
		return "", fmt.Errorf("failed to create DAGs directory %s", d.dir)
	}
	loc, err := d.fileLocation(name)
	if err != nil {
		return "", fmt.Errorf("failed to create DAG file: %s", err)
	}
	if exists(loc) {
		return "", fmt.Errorf("the DAG file %s already exists", loc)
	}
	return name, os.WriteFile(loc, spec, 0644)
}

func (d *dagStoreImpl) Delete(name string) error {
	loc, err := d.fileLocation(name)
	if err != nil {
		return fmt.Errorf("failed to create DAG file: %s", err)
	}
	err = os.Remove(loc)
	if err != nil {
		return fmt.Errorf("failed to delete DAG file: %s", err)
	}
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
	dl := &dag.Loader{}
	opts := &grep.Options{
		IsRegexp: true,
		Before:   2,
		After:    2,
	}

	utils.LogErr("read DAGs directory", err)
	for _, fi := range fis {
		if utils.MatchExtension(fi.Name(), dag.EXTENSIONS) {
			file := filepath.Join(d.dir, fi.Name())
			dat, err := os.ReadFile(file)
			if err != nil {
				utils.LogErr("read DAG file", err)
				continue
			}
			m, err := grep.Grep(dat, fmt.Sprintf("(?i)%s", pattern), opts)
			if err != nil {
				errs = append(errs, fmt.Sprintf("grep %s failed: %s", fi.Name(), err))
				continue
			}
			d, err := dl.LoadMetadata(file)
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
	//TODO implement me
	panic("implement me")
}

func (d *dagStoreImpl) Rename(oldDAGPath, newDAGPath string) error {
	oldLoc, err := d.fileLocation(oldDAGPath)
	if err != nil {
		return fmt.Errorf("invalid old name: %s", oldDAGPath)
	}
	newLoc, err := d.fileLocation(newDAGPath)
	if err != nil {
		return fmt.Errorf("invalid new name: %s", newDAGPath)
	}
	if err := os.Rename(oldLoc, newLoc); err != nil {
		return err
	}
	return nil
}
