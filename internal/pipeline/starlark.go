package pipeline

import (
	"fmt"
	"go.starlark.net/starlarkstruct"
	"log"
	"path/filepath"

	"go.starlark.net/lib/time"
	"go.starlark.net/starlark"
)

// MakeLoad returns a simple sequential implementation of module loading
// suitable for use in the REPL.
// Each function returned by MakeLoad accesses a distinct private cache.
func MakeLoad(predeclared starlark.StringDict) func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
	type entry struct {
		globals starlark.StringDict
		err     error
	}

	var cache = make(map[string]*entry)

	return func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
		e, ok := cache[module]
		if e == nil {
			if ok {
				// request for package whose loading is in progress
				return nil, fmt.Errorf("cycle in load graph")
			}

			// Add a placeholder to indicate "load in progress".
			cache[module] = nil

			// Load it.
			thread := &starlark.Thread{Name: "exec " + module, Load: thread.Load}
			file, err := filepath.Abs(module)
			if err != nil {
				return nil, err
			}
			globals, err := starlark.ExecFile(thread, file, nil, predeclared)
			e = &entry{globals, err}

			// Update the cache.
			cache[module] = e
		}
		return e.globals, e.err
	}
}

func NewPipeline(path string) (starlark.StringDict, error) {
	graph := starlark.StringDict{
		"nodes": &starlark.Dict{},
		"edges": &starlark.List{},
	}
	predeclared := starlark.StringDict{
		"time": time.Module,
		"graph": &starlarkstruct.Module{
			Name:    "graph",
			Members: graph,
		},
	}
	// Execute Starlark program in a file.
	thread := &starlark.Thread{
		Name: "default",
		Load: MakeLoad(predeclared), // For a more powerful loader, refer to [this](https://github.com/cirruslabs/cirrus-cli/blob/master/pkg/larker/loader/loader.go#L45).
	}

	globals, err := starlark.ExecFile(thread, path, nil, predeclared)
	if err != nil {
		return nil, err
	}

	// Retrieve a module global.
	main := globals["main"]

	// Call Starlark function from Go.
	v, err := starlark.Call(thread, main, starlark.Tuple{starlark.MakeInt(100)}, nil)
	if err != nil {
		return nil, err
	}
	if v != starlark.MakeInt(0) {
		fmt.Printf("exit code: %v", v)
	}
	return graph, nil
}

func EdgesToDependencyMap(edges *starlark.List) map[string][]string {
	dependencyMap := map[string][]string{}
	for i := 0; i < edges.Len(); i++ {
		edge := edges.Index(i).(starlark.Tuple)
		src, ok := starlark.AsString(edge.Index(0))
		if !ok {
			log.Panicf("edge must be a pair of string")
		}
		dst, ok := starlark.AsString(edge.Index(1))
		if !ok {
			log.Panicf("edge must be a pair of string")
		}
		dependencyMap[dst] = append(dependencyMap[dst], src)
	}
	return dependencyMap
}
