package intg_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
)

func TestDollarEscape(t *testing.T) {
	t.Parallel()

	t.Run("BackslashDollarLiteralInJQ", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `
env:
  - PRICE: '\$9.99'
steps:
  - name: jq-price
    type: jq
    config:
      raw: true
    script: |
      {"price":"${PRICE}"}
    command: ".price"
    output: PRICE_OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"PRICE_OUT": "$9.99",
		})
	})

	t.Run("SingleQuotedVarPreservesQuotes", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `
steps:
  - name: jq-literal
    type: jq
    config:
      raw: true
    script: |
      {"literal":"'${HOME}'"}
    command: ".literal"
    output: LITERAL_OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"LITERAL_OUT": "'${HOME}'",
		})
	})

	t.Run("BackslashDollarLiteralInHTTPBody", func(t *testing.T) {
		t.Parallel()

		bodyCh := make(chan string, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() { _ = r.Body.Close() }()
			body, err := io.ReadAll(r.Body)
			if err == nil {
				bodyCh <- string(body)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}))
		defer server.Close()

		th := test.Setup(t)
		dag := th.DAG(t, fmt.Sprintf(`
steps:
  - name: http-price
    type: http
    config:
      headers:
        Content-Type: application/json
      body: '{"price":"\$9.99"}'
      silent: true
    command: POST %s/price
`, server.URL))
		agent := dag.Agent()
		agent.RunSuccess(t)

		select {
		case got := <-bodyCh:
			if got != `{"price":"$9.99"}` {
				t.Fatalf("expected body to contain $9.99, got %q", got)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for HTTP body")
		}
	})
}
