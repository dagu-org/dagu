// Copyright (C) 2024 The Daguflow/Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package dag

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/daguflow/dagu/internal/constants"
)

// Finder finds a DAG by name.
// This is used to find the DAG when a node references another DAG.
type Finder interface {
	Find(name string) (*DAG, error)
}

// Context contains the current DAG and Finder.
type Context struct {
	DAG    *DAG
	Finder Finder
	Envs   []string
}

// ctxKey is used as the key for storing the DAG in the context.
type ctxKey struct{}

// NewContext creates a new context with the DAG and Finder.
func NewContext(ctx context.Context, dag *DAG, finder Finder) context.Context {
	return context.WithValue(ctx, ctxKey{}, Context{DAG: dag, Finder: finder, Envs: make([]string, 0)})
}

var (
	errFailedCtxAssertion = errors.New("failed to assert DAG context")
)

// GetContext returns the DAG Context from the context.
// It returns an error if the context does not contain a DAG Context.
func GetContext(ctx context.Context) (Context, error) {
	dagCtx, ok := ctx.Value(ctxKey{}).(Context)
	if !ok {
		return Context{}, errFailedCtxAssertion
	}
	return dagCtx, nil
}

func WithDagContext(ctx context.Context, dagContext Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, dagContext)
}

func GenGlobalStepLogEnvKey(stepID int) string {
	var (
		keyBuilder = strings.Builder{}
	)

	keyBuilder.WriteString(constants.StepDaguExecutionLogPathKeyPrefix)

	keyBuilder.WriteString("_")

	keyBuilder.WriteString(strconv.Itoa(stepID))

	keyBuilder.WriteString("_")

	keyBuilder.WriteString(constants.StepDaguExecutionLogPathKeySuffix)

	return keyBuilder.String()
}
