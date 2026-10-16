// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/dagucloud/dagu/v2/internal/runtime/builtin/internal/agentstep"
)

// An upload gives local files to a file input of the page. Without into, the
// page's only file input gets them. With into, the model finds the element
// into describes, once: the action is recorded like an act's and replayed on
// later runs. The element gets the files directly when it is a file input
// or holds exactly one; otherwise it is clicked and the file picker that
// opens gets them, which the browser intercepts so no window shows; when no
// picker opens, the one file input nearest the element gets them.

const (
	// maxUploadFileBytes bounds each file an upload gives the page.
	maxUploadFileBytes = 50 << 20
	// uploadMethod is the method of an upload's recorded action.
	uploadMethod = "upload"
	// uploadKeyPrefix sets an upload's recordings apart from those of an act
	// with the same words.
	uploadKeyPrefix = "upload\x00"
)

// How an upload's files reached a file input.
const (
	// uploadDirect gave them to the element, or the file input it holds.
	uploadDirect = "direct"
	// uploadPicker gave them to the file picker a click on the element
	// opened.
	uploadPicker = "picker"
	// uploadNearby gave them to the file input nearest the element.
	uploadNearby = "nearby"
)

var errNoFileInput = errors.New("the page has no file input; with into, name the button or area that opens the file picker")

// errSingleFile reports a file input or picker that takes one file, given
// several.
type errSingleFile struct{ files int }

func (e errSingleFile) Error() string {
	return fmt.Sprintf("the file input takes one file, and the upload gives %d; upload them one at a time", e.files)
}

// errFileInputs reports a page with several file inputs and no into to say
// which one to use.
type errFileInputs struct{ count int }

func (e errFileInputs) Error() string {
	return fmt.Sprintf("the page has %d file inputs; with into, say which one to use", e.count)
}

// uploadFile is a file an upload gives the page.
type uploadFile struct {
	// Path is absolute.
	Path    string
	Size    int64
	ModTime time.Time
}

// uploadReplayKey identifies an upload operation on a page in the replay
// cache.
func uploadReplayKey(index int, into, pageURL string) string {
	return replayKey(index, uploadKeyPrefix+into, pageURL)
}

// uploadInstruction is what the model is asked to find for an upload into
// the element into describes.
func uploadInstruction(into string) string {
	return "Find the one element to upload files with: " + into +
		". It is a file input, or the button, link, or drop area that opens a file picker or holds a file input."
}

func (o *operator) upload(ctx context.Context, index int, spec uploadSpec, timeout time.Duration) (opResult, error) {
	began, before := time.Now(), o.bridge.totals()
	files, err := resolveUploadFiles(spec.Files, o.workDir)
	if err != nil {
		return opResult{}, err
	}
	result := func(status, via string, action *recordedAction, how, pageURL string) opResult {
		done := opResult{
			Report: agentstep.Report{
				Index: index, Kind: opUpload, Subject: uploadSubject(spec), Status: status, Via: via,
				Detail: describeUpload(files, action, how), Tokens: o.bridge.totals().sub(before).total(),
				Duration: time.Since(began),
			},
			PageURL: pageURL,
		}
		if action != nil {
			done.Actions = []recordedAction{*action}
		}
		return done
	}
	if spec.Into == "" {
		how, err := o.eng.Upload(ctx, "", files, timeout)
		if err != nil {
			return opResult{}, err
		}
		return result(agentstep.StatusCompleted, "", nil, how, ""), nil
	}

	choice, recorded := o.decide(spec.AI, spec.Cache)
	key, pageURL := "", ""
	if recorded {
		if pageURL, err = o.eng.CurrentURL(ctx); err != nil {
			return opResult{}, err
		}
		key = uploadReplayKey(index, spec.Into, pageURL)
	}
	// The page before the upload identifies the element the model picks,
	// and tells whether a recorded element is still the one recorded.
	snap, snapErr := o.eng.Snapshot(ctx)
	status := agentstep.StatusCompleted
	actions, ok := o.recording(choice, key)
	if !ok && choice == aiNever {
		return opResult{}, replayMiss{kind: opUpload, reason: noRecording}
	}
	if ok {
		action := actions[0]
		if snapErr == nil && stillTargets(action, snap, o.variables) {
			how, err := o.eng.Upload(ctx, action.Selector, files, timeout)
			if err != nil {
				return opResult{}, err
			}
			return result(agentstep.StatusCacheHit, agentstep.ViaElement, &action, how, pageURL), nil
		}
		if choice == aiNever {
			reason := fmt.Sprintf("its recorded element %s is gone or no longer the one recorded", action.Selector)
			if snapErr != nil {
				reason = "the page could not be read: " + snapErr.Error()
			}
			return opResult{}, replayMiss{kind: opUpload, reason: reason}
		}
		status = agentstep.StatusHealed
		// The recording is dropped unless the upload that heals it records
		// what it did.
		o.recorder.drop(key)
	}
	found, err := o.eng.Observe(ctx, uploadInstruction(spec.Into), timeout)
	if err != nil {
		return opResult{}, err
	}
	if len(found) == 0 {
		return opResult{}, fmt.Errorf("the model (%s) answered that no element on the page matches %q; "+
			"if the element is on the page, reword into or try another model", o.bridge.modelName(), spec.Into)
	}
	action := recordedAction{Selector: found[0].Selector, Description: found[0].Description, Method: uploadMethod}
	if target, ok := targetOf(snap, action.Selector); ok && snapErr == nil {
		action = identified(action, target, o.variables)
	}
	how, err := o.eng.Upload(ctx, action.Selector, files, timeout)
	if err != nil {
		return opResult{}, err
	}
	if recorded {
		o.recorder.stage(key, []recordedAction{action})
	}
	return result(status, agentstep.ViaModel, &action, how, pageURL), nil
}

// uploadSubject names an upload's file input as its report does.
func uploadSubject(spec uploadSpec) string {
	if spec.Into == "" {
		return "the page's file input"
	}
	return spec.Into
}

// describeUpload says which files an upload gave and how they reached the
// page.
func describeUpload(files []uploadFile, action *recordedAction, how string) string {
	names := make([]string, 0, len(files))
	for _, file := range files {
		names = append(names, filepath.Base(file.Path))
	}
	list := strings.Join(names, ", ")
	if action == nil {
		return list + " into the page's file input"
	}
	switch how {
	case uploadPicker:
		return list + " through the file picker " + action.Selector + " opened"
	case uploadNearby:
		return list + " into the file input nearest " + action.Selector
	default:
		return list + " into " + action.Selector
	}
}

// resolveUploadFiles returns the files upload.files names, in order and
// each once. An entry is a path or a glob pattern, several of them one per
// line, or JSON as an earlier step publishes it: a list of paths, of objects
// with a path such as saved mail attachments, or of emails from mail.search,
// whose saved attachments it names, or one such object. Relative paths
// resolve against workDir, or the current directory when it is empty.
func resolveUploadFiles(entries []string, workDir string) ([]uploadFile, error) {
	var files []uploadFile
	seen := map[string]bool{}
	for _, entry := range entries {
		paths, err := uploadPaths(entry)
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			matches, err := expandUploadPath(path, workDir)
			if err != nil {
				return nil, err
			}
			for _, match := range matches {
				if seen[match] {
					continue
				}
				seen[match] = true
				file, err := statUploadFile(match)
				if err != nil {
					return nil, err
				}
				files = append(files, file)
			}
		}
	}
	if len(files) == 0 {
		return nil, errors.New("files names no file")
	}
	return files, nil
}

// uploadPaths returns the paths and patterns an entry of upload.files
// names.
func uploadPaths(entry string) ([]string, error) {
	text := strings.TrimSpace(entry)
	if strings.HasPrefix(text, "[") || strings.HasPrefix(text, "{") {
		var value any
		if json.Unmarshal([]byte(text), &value) == nil {
			return jsonUploadPaths(value)
		}
	}
	var paths []string
	for line := range strings.Lines(text) {
		if path := strings.TrimSpace(line); path != "" {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

// jsonUploadPaths returns the paths a JSON value lists.
func jsonUploadPaths(value any) ([]string, error) {
	items, isList := value.([]any)
	if !isList {
		items = []any{value}
	}
	paths := make([]string, 0, len(items))
	for _, item := range items {
		switch item := item.(type) {
		case string:
			if strings.TrimSpace(item) != "" {
				paths = append(paths, strings.TrimSpace(item))
			}
		case map[string]any:
			// An email without attachments lists them as null.
			if attachments, isEmail := item["attachments"]; isEmail {
				list, _ := attachments.([]any)
				saved, err := jsonUploadPaths(list)
				if err != nil {
					return nil, err
				}
				paths = append(paths, saved...)
				continue
			}
			path, _ := item["path"].(string)
			if strings.TrimSpace(path) == "" {
				name, _ := item["name"].(string)
				return nil, fmt.Errorf("files lists %s without a path; save mail attachments with save_attachments: true", listedName(name))
			}
			paths = append(paths, strings.TrimSpace(path))
		default:
			return nil, fmt.Errorf("files lists %v, which is neither a path nor an object with a path", item)
		}
	}
	return paths, nil
}

// listedName names a listed object by its name, if it has one.
func listedName(name string) string {
	if name == "" {
		return "an object"
	}
	return fmt.Sprintf("%q", name)
}

// expandUploadPath returns the file at path, or the files a glob pattern
// matches, sorted.
func expandUploadPath(path, workDir string) ([]string, error) {
	resolved, err := absUploadPath(path, workDir)
	if err != nil {
		return nil, err
	}
	if _, err := os.Lstat(resolved); err == nil || !strings.ContainsAny(path, "*?[") {
		return []string{resolved}, nil
	}
	matches, err := filepath.Glob(resolved)
	if err != nil {
		return nil, fmt.Errorf("files pattern %s: %w", path, err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no file matches %s", resolved)
	}
	slices.Sort(matches)
	return matches, nil
}

// absUploadPath makes path absolute: a leading ~ is the home directory, and
// a relative path is under workDir.
func absUploadPath(path, workDir string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", path, err)
		}
		return filepath.Join(home, path[1:]), nil
	}
	if !filepath.IsAbs(path) && workDir != "" {
		path = filepath.Join(workDir, path)
	}
	return filepath.Abs(path)
}

// statUploadFile checks that path is a file an upload can give.
func statUploadFile(path string) (uploadFile, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return uploadFile{}, fmt.Errorf("file %s does not exist", path)
	}
	if err != nil {
		return uploadFile{}, err
	}
	if info.IsDir() {
		return uploadFile{}, fmt.Errorf("%s is a directory; name the files in it, such as %s", path, filepath.Join(path, "*.pdf"))
	}
	if !info.Mode().IsRegular() {
		return uploadFile{}, fmt.Errorf("%s is not a regular file", path)
	}
	if info.Size() > maxUploadFileBytes {
		return uploadFile{}, fmt.Errorf("%s is %d MB; an upload takes files of up to %d MB", path, (info.Size()+(1<<20)-1)>>20, maxUploadFileBytes>>20)
	}
	return uploadFile{Path: path, Size: info.Size(), ModTime: info.ModTime()}, nil
}
