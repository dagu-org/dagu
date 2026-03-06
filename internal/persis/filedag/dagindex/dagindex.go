package dagindex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec"
	indexv1 "github.com/dagu-org/dagu/proto/index/v1"
	"github.com/robfig/cron/v3"
	"google.golang.org/protobuf/proto"
)

var cronParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
)

const (
	// IndexFileName is the name of the DAG definition index file.
	IndexFileName = ".dag.index"
	// IndexVersion is the current index format version.
	IndexVersion = 1
)

// YAMLFileMeta holds stat metadata for a single YAML file.
type YAMLFileMeta struct {
	Name    string // filename, e.g. "my-dag.yaml"
	Size    int64
	ModTime int64 // UnixNano
}

// SuspendFlags is the set of suspend flag filenames present in flagsBaseDir.
type SuspendFlags map[string]struct{}

// Load reads and validates the index against the current filesystem state.
// Returns nil if the index is missing, corrupt, version-mismatched, or stale.
func Load(indexPath string, yamlFiles []YAMLFileMeta, flags SuspendFlags) []*indexv1.DAGIndexEntry {
	data, err := os.ReadFile(indexPath) //nolint:gosec
	if err != nil {
		return nil
	}

	var idx indexv1.DAGIndex
	if err := proto.Unmarshal(data, &idx); err != nil {
		return nil
	}

	if idx.Version != IndexVersion {
		return nil
	}

	if len(idx.Entries) != len(yamlFiles) {
		return nil
	}

	// Build lookup by file_path for O(n) comparison.
	entryMap := make(map[string]*indexv1.DAGIndexEntry, len(idx.Entries))
	for _, e := range idx.Entries {
		entryMap[e.FilePath] = e
	}

	for _, f := range yamlFiles {
		e, ok := entryMap[f.Name]
		if !ok {
			return nil
		}
		if e.FileSize != f.Size || e.ModTime != f.ModTime {
			return nil
		}
	}

	// Validate suspend flags.
	for _, e := range idx.Entries {
		_, flagged := flags[SuspendFlagName(e.Name)]
		if e.Suspended != flagged {
			return nil
		}
	}

	return idx.Entries
}

// Build constructs a fresh index by loading every YAML file with metadata-only semantics.
func Build(ctx context.Context, dagDir string, yamlFiles []YAMLFileMeta, flags SuspendFlags) *indexv1.DAGIndex {
	idx := &indexv1.DAGIndex{
		Version:     IndexVersion,
		BuiltAtUnix: time.Now().Unix(),
		Entries:     make([]*indexv1.DAGIndexEntry, 0, len(yamlFiles)),
	}

	for _, f := range yamlFiles {
		if ctx.Err() != nil {
			break
		}

		filePath := filepath.Join(dagDir, f.Name)
		entry := &indexv1.DAGIndexEntry{
			FilePath: f.Name,
			FileSize: f.Size,
			ModTime:  f.ModTime,
		}

		dag, err := spec.Load(ctx, filePath,
			spec.OnlyMetadata(),
			spec.WithoutEval(),
			spec.SkipSchemaValidation(),
			spec.WithAllowBuildErrors(),
		)
		if err != nil {
			entry.Name = strings.TrimSuffix(f.Name, filepath.Ext(f.Name))
			entry.LoadError = err.Error()
			idx.Entries = append(idx.Entries, entry)
			continue
		}

		entry.Name = dag.Name
		entry.Group = dag.Group
		entry.Description = dag.Description
		entry.Tags = tagsToStrings(dag.Tags)
		entry.Schedule = scheduleToString(dag.Schedule)

		if len(dag.BuildErrors) > 0 {
			entry.LoadError = joinErrors(dag.BuildErrors)
		}

		_, flagged := flags[SuspendFlagName(dag.Name)]
		entry.Suspended = flagged

		idx.Entries = append(idx.Entries, entry)
	}

	return idx
}

// Write atomically writes the index to disk.
func Write(indexPath string, idx *indexv1.DAGIndex) error {
	data, err := proto.Marshal(idx)
	if err != nil {
		return fmt.Errorf("failed to marshal DAG index: %w", err)
	}
	return fileutil.WriteFileAtomic(indexPath, data, 0600)
}

// DAGFromEntry reconstructs a minimal core.DAG from an index entry.
// The returned DAG is suitable for List/TagList operations.
func DAGFromEntry(entry *indexv1.DAGIndexEntry, baseDir string) *core.DAG {
	dag := &core.DAG{
		Name:        entry.Name,
		Location:    filepath.Join(baseDir, entry.FilePath),
		Group:       entry.Group,
		Description: entry.Description,
		Tags:        core.NewTags(entry.Tags),
	}

	if entry.LoadError != "" {
		dag.BuildErrors = []error{errors.New(entry.LoadError)}
	}

	if entry.Schedule != "" {
		dag.Schedule = parseScheduleExpressions(entry.Schedule)
	}

	return dag
}

// SuspendFlagName returns the flag filename for a DAG name.
func SuspendFlagName(dagName string) string {
	return fileutil.NormalizeFilename(dagName, "-") + ".suspend"
}

func tagsToStrings(tags core.Tags) []string {
	if len(tags) == 0 {
		return nil
	}
	strs := make([]string, len(tags))
	for i, t := range tags {
		strs[i] = t.String()
	}
	return strs
}

// scheduleToString joins schedule expressions with "; " as a delimiter.
// This is safe because cron expressions never contain semicolons.
func scheduleToString(schedules []core.Schedule) string {
	if len(schedules) == 0 {
		return ""
	}
	exprs := make([]string, len(schedules))
	for i, s := range schedules {
		exprs[i] = s.Expression
	}
	return strings.Join(exprs, "; ")
}

func parseScheduleExpressions(s string) []core.Schedule {
	parts := strings.Split(s, "; ")
	var schedules []core.Schedule
	for _, expr := range parts {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			continue
		}
		sched := core.Schedule{Expression: expr}
		// Parse the cron expression for NextRun support.
		// Failure is non-fatal: the schedule just won't have a Parsed field.
		if parsed, err := cronParser.Parse(expr); err == nil {
			sched.Parsed = parsed
		}
		schedules = append(schedules, sched)
	}
	return schedules
}

func joinErrors(errs []error) string {
	strs := make([]string, len(errs))
	for i, e := range errs {
		strs[i] = e.Error()
	}
	return strings.Join(strs, "; ")
}
