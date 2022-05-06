package sock

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSockAddr(t *testing.T) {
	val := GetSockAddr("test")
	require.Regexp(t, `^/tmp/@dagu-test-[0-9a-f]+\.sock$`, val)
}
