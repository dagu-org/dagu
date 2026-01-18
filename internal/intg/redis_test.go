package intg_test

import (
	"fmt"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
)

const redisTestImage = "redis:7-alpine"

type redisTest struct {
	name            string
	dagConfigFunc   func(port int) string
	expectedOutputs map[string]any
}

func TestDAGLevelRedis(t *testing.T) {
	t.Parallel()

	// Base port - each test gets its own port to allow parallel execution
	basePort := 16379

	tests := []redisTest{
		{
			name: "BasicPing",
			dagConfigFunc: func(port int) string {
				return fmt.Sprintf(`
container:
  image: %s
  startup: entrypoint
  ports:
    - "%d:6379"

redis:
  host: localhost
  port: %d

steps:
  - name: ping
    type: redis
    config:
      command: PING
    output: REDIS_OUT
`, redisTestImage, port, port)
			},
			expectedOutputs: map[string]any{
				"REDIS_OUT": "\"PONG\"", // JSON-encoded string
			},
		},
		{
			name: "SetAndGet",
			dagConfigFunc: func(port int) string {
				return fmt.Sprintf(`
container:
  image: %s
  startup: entrypoint
  ports:
    - "%d:6379"

redis:
  host: localhost
  port: %d

steps:
  - name: set-value
    type: redis
    config:
      command: SET
      key: test-key
      value: hello
  - name: get-value
    depends:
      - set-value
    type: redis
    config:
      command: GET
      key: test-key
    output: REDIS_GET_OUT
`, redisTestImage, port, port)
			},
			expectedOutputs: map[string]any{
				"REDIS_GET_OUT": "\"hello\"", // JSON-encoded string
			},
		},
		{
			name: "StepOverridesDB",
			dagConfigFunc: func(port int) string {
				return fmt.Sprintf(`
container:
  image: %s
  startup: entrypoint
  ports:
    - "%d:6379"

redis:
  host: localhost
  port: %d
  db: 0

steps:
  - name: set-in-db1
    type: redis
    config:
      command: SET
      key: db-test-key
      value: in-db1
      db: 1
  - name: get-from-db1
    depends:
      - set-in-db1
    type: redis
    config:
      command: GET
      key: db-test-key
      db: 1
    output: REDIS_DB1_OUT
`, redisTestImage, port, port)
			},
			expectedOutputs: map[string]any{
				"REDIS_DB1_OUT": "\"in-db1\"",
			},
		},
		{
			// Pipeline test - just verify it succeeds, output format is complex
			name: "Pipeline",
			dagConfigFunc: func(port int) string {
				return fmt.Sprintf(`
container:
  image: %s
  startup: entrypoint
  ports:
    - "%d:6379"

redis:
  host: localhost
  port: %d

steps:
  - name: run-pipeline
    type: redis
    config:
      pipeline:
        - command: SET
          key: pipe-key1
          value: value1
        - command: SET
          key: pipe-key2
          value: value2
        - command: GET
          key: pipe-key1
        - command: GET
          key: pipe-key2
`, redisTestImage, port, port)
			},
			expectedOutputs: nil, // Just verify it succeeds
		},
		{
			// List operations - just verify it succeeds
			name: "ListOperations",
			dagConfigFunc: func(port int) string {
				return fmt.Sprintf(`
container:
  image: %s
  startup: entrypoint
  ports:
    - "%d:6379"

redis:
  host: localhost
  port: %d

steps:
  - name: lpush-items
    type: redis
    config:
      command: LPUSH
      key: mylist
      values:
        - item1
        - item2
        - item3
  - name: llen
    depends:
      - lpush-items
    type: redis
    config:
      command: LLEN
      key: mylist
    output: LIST_LEN
`, redisTestImage, port, port)
			},
			expectedOutputs: map[string]any{
				"LIST_LEN": "3", // Integer output
			},
		},
		{
			// Hash operations - just verify it succeeds
			name: "HashOperations",
			dagConfigFunc: func(port int) string {
				return fmt.Sprintf(`
container:
  image: %s
  startup: entrypoint
  ports:
    - "%d:6379"

redis:
  host: localhost
  port: %d

steps:
  - name: hset-fields
    type: redis
    config:
      command: HSET
      key: myhash
      fields:
        field1: value1
        field2: value2
  - name: hget-single
    depends:
      - hset-fields
    type: redis
    config:
      command: HGET
      key: myhash
      field: field1
    output: HASH_OUT
`, redisTestImage, port, port)
			},
			expectedOutputs: map[string]any{
				"HASH_OUT": "\"value1\"",
			},
		},
		{
			name: "SetWithTTL",
			dagConfigFunc: func(port int) string {
				return fmt.Sprintf(`
container:
  image: %s
  startup: entrypoint
  ports:
    - "%d:6379"

redis:
  host: localhost
  port: %d

steps:
  - name: set-with-ttl
    type: redis
    config:
      command: SET
      key: ttl-key
      value: ttl-value
      ttl: 3600
  - name: exists-check
    depends:
      - set-with-ttl
    type: redis
    config:
      command: EXISTS
      key: ttl-key
    output: EXISTS_OUT
`, redisTestImage, port, port)
			},
			expectedOutputs: map[string]any{
				"EXISTS_OUT": "1", // Key exists
			},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			port := basePort + i
			th := test.Setup(t)
			dag := th.DAG(t, tt.dagConfigFunc(port))
			dag.Agent().RunSuccess(t)
			dag.AssertLatestStatus(t, core.Succeeded)
			if len(tt.expectedOutputs) > 0 {
				dag.AssertOutputs(t, tt.expectedOutputs)
			}
		})
	}
}
