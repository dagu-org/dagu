package prototype

import (
	"testing"

	"github.com/dagu-org/dagu/internal/test"
)

func TestQueue(t *testing.T) {
	t.Parallel()

	t.Run("CreateQueue", func(t *testing.T) {
		th := test.Setup(t)
		_ = &Queue{
			baseDir: th.Config.Paths.QueueDir,
		}
	})
}
