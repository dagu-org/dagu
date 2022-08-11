package runner

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/storage"
	"github.com/yohamta/dagu/internal/suspend"
	"github.com/yohamta/dagu/internal/utils"
)

type Entry struct {
	Next time.Time
	Job  Job
}

type EntryReader interface {
	Read(now time.Time) ([]*Entry, error)
}

func NewEntryReader(cfg *admin.Config) *entryReader {
	return &entryReader{
		Admin: cfg,
		suspendChecker: suspend.NewSuspendChecker(
			storage.NewStorage(
				settings.MustGet(settings.SETTING__SUSPEND_FLAGS_DIR),
			),
		),
	}
}

type entryReader struct {
	Admin          *admin.Config
	suspendChecker *suspend.SuspendChecker
}

var _ EntryReader = (*entryReader)(nil)

func (er *entryReader) Read(now time.Time) ([]*Entry, error) {
	cl := dag.Loader{}
	entries := []*Entry{}
	for {
		fis, err := os.ReadDir(er.Admin.DAGs)
		if err != nil {
			return nil, fmt.Errorf("failed to read entries directory: %w", err)
		}
		for _, fi := range fis {
			if utils.MatchExtension(fi.Name(), dag.EXTENSIONS) {
				dag, err := cl.LoadHeadOnly(
					filepath.Join(er.Admin.DAGs, fi.Name()),
				)
				if err != nil {
					log.Printf("failed to read dag config: %s", err)
					continue
				}
				if er.suspendChecker.IsSuspended(dag) {
					continue
				}
				for _, sc := range dag.Schedule {
					next := sc.Next(now)
					entries = append(entries, &Entry{
						Next: sc.Next(now),
						Job: &job{
							DAG:       dag,
							Config:    er.Admin,
							StartTime: next,
						},
					})
				}
			}
		}
		return entries, nil
	}
}
