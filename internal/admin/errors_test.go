package admin

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeError(t *testing.T) {
	rw := &mockResponseWriter{}
	encodeError(rw, errNotFound)
	require.Equal(t, http.StatusNotFound, rw.status)

	rw = &mockResponseWriter{}
	encodeError(rw, fmt.Errorf("test error"))
	require.Equal(t, http.StatusInternalServerError, rw.status)
}
