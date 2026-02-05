package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/itchyny/gojq"
)

// resolveJSONPath extracts a value from JSON data using a jq-style path.
func resolveJSONPath(ctx context.Context, varName, jsonStr, path string) (string, bool) {
	var raw any
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		logger.Warn(ctx, "Failed to parse JSON",
			slog.String("var", varName),
			tag.Error(err))
		return "", false
	}

	query, err := gojq.Parse(path)
	if err != nil {
		logger.Warn(ctx, "Failed to parse path in data",
			tag.Path(path),
			slog.String("var", varName),
			tag.Error(err))
		return "", false
	}

	iter := query.Run(raw)
	v, ok := iter.Next()
	if !ok {
		return "", false
	}

	if err, ok := v.(error); ok {
		logger.Warn(ctx, "Error evaluating path in data",
			tag.Path(path),
			slog.String("var", varName),
			tag.Error(err))
		return "", false
	}

	return fmt.Sprintf("%v", v), true
}
