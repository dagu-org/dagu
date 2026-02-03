package builtin

import (
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/archive"
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/chat"
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/command"
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/dag"
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/docker"
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/gha"
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/hitl"
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/http"
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/jq"
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/mail"
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/redis"
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/router"
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/s3"
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/sql"
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/sql/drivers/postgres"
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/sql/drivers/sqlite"
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/ssh"
)
