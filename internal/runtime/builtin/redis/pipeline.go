package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// PipelineExecutor executes pipeline/transaction commands.
type PipelineExecutor struct {
	client goredis.UniversalClient
	cfg    *Config
}

// NewPipelineExecutor creates a new pipeline executor.
func NewPipelineExecutor(client goredis.UniversalClient, cfg *Config) *PipelineExecutor {
	return &PipelineExecutor{client: client, cfg: cfg}
}

// Execute executes the pipeline commands and returns results.
func (e *PipelineExecutor) Execute(ctx context.Context) ([]any, error) {
	if len(e.cfg.Pipeline) == 0 {
		return nil, fmt.Errorf("no pipeline commands specified")
	}

	var pipe goredis.Pipeliner

	// Use WATCH if specified (for optimistic locking)
	if len(e.cfg.Watch) > 0 {
		return e.executeWithWatch(ctx)
	}

	// Use MULTI/EXEC if configured
	if e.cfg.Multi {
		pipe = e.client.TxPipeline()
	} else {
		pipe = e.client.Pipeline()
	}

	// Queue commands
	cmds := make([]goredis.Cmder, len(e.cfg.Pipeline))
	for i, cmd := range e.cfg.Pipeline {
		cmder, err := e.queueCommand(ctx, pipe, &cmd)
		if err != nil {
			return nil, fmt.Errorf("failed to queue command %d (%s): %w", i, cmd.Command, err)
		}
		cmds[i] = cmder
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil && err != goredis.Nil {
		return nil, fmt.Errorf("pipeline execution failed: %w", err)
	}

	// Collect results
	results := make([]any, len(cmds))
	for i, cmd := range cmds {
		result, err := extractCmdResult(cmd)
		if err != nil && err != goredis.Nil {
			results[i] = map[string]any{
				"error": err.Error(),
			}
		} else {
			results[i] = result
		}
	}

	return results, nil
}

// executeWithWatch executes commands with WATCH for optimistic locking.
func (e *PipelineExecutor) executeWithWatch(ctx context.Context) ([]any, error) {
	var results []any

	err := e.client.Watch(ctx, func(tx *goredis.Tx) error {
		pipe := tx.TxPipeline()

		// Queue commands
		cmds := make([]goredis.Cmder, len(e.cfg.Pipeline))
		for i, cmd := range e.cfg.Pipeline {
			cmder, err := e.queueCommand(ctx, pipe, &cmd)
			if err != nil {
				return fmt.Errorf("failed to queue command %d (%s): %w", i, cmd.Command, err)
			}
			cmds[i] = cmder
		}

		// Execute
		_, err := pipe.Exec(ctx)
		if err != nil && err != goredis.Nil {
			return err
		}

		// Collect results
		results = make([]any, len(cmds))
		for i, cmd := range cmds {
			result, err := extractCmdResult(cmd)
			if err != nil && err != goredis.Nil {
				results[i] = map[string]any{
					"error": err.Error(),
				}
			} else {
				results[i] = result
			}
		}

		return nil
	}, e.cfg.Watch...)

	if err != nil {
		return nil, fmt.Errorf("watch transaction failed: %w", err)
	}

	return results, nil
}

// queueCommand queues a single command to the pipeline.
func (e *PipelineExecutor) queueCommand(_ context.Context, pipe goredis.Pipeliner, cmd *PipelineCommand) (goredis.Cmder, error) {
	switch strings.ToUpper(cmd.Command) {
	// String commands
	case "GET":
		return pipe.Get(context.Background(), cmd.Key), nil
	case "SET":
		ttl := time.Duration(cmd.TTL) * time.Second
		if cmd.NX {
			return pipe.SetNX(context.Background(), cmd.Key, cmd.Value, ttl), nil
		}
		if cmd.XX {
			return pipe.SetXX(context.Background(), cmd.Key, cmd.Value, ttl), nil
		}
		return pipe.Set(context.Background(), cmd.Key, cmd.Value, ttl), nil
	case "MGET":
		return pipe.MGet(context.Background(), cmd.Keys...), nil
	case "MSET":
		return pipe.MSet(context.Background(), cmd.Values...), nil
	case "INCR":
		return pipe.Incr(context.Background(), cmd.Key), nil
	case "DECR":
		return pipe.Decr(context.Background(), cmd.Key), nil

	// Key commands
	case "DEL":
		keys := cmd.Keys
		if len(keys) == 0 && cmd.Key != "" {
			keys = []string{cmd.Key}
		}
		return pipe.Del(context.Background(), keys...), nil
	case "EXISTS":
		keys := cmd.Keys
		if len(keys) == 0 && cmd.Key != "" {
			keys = []string{cmd.Key}
		}
		return pipe.Exists(context.Background(), keys...), nil
	case "EXPIRE":
		return pipe.Expire(context.Background(), cmd.Key, time.Duration(cmd.TTL)*time.Second), nil
	case "TTL":
		return pipe.TTL(context.Background(), cmd.Key), nil

	// Hash commands
	case "HGET":
		return pipe.HGet(context.Background(), cmd.Key, cmd.Field), nil
	case "HSET":
		args := make([]any, 0, len(cmd.Fields)*2)
		for k, v := range cmd.Fields {
			args = append(args, k, v)
		}
		if cmd.Field != "" && cmd.Value != nil {
			args = append(args, cmd.Field, cmd.Value)
		}
		return pipe.HSet(context.Background(), cmd.Key, args...), nil
	case "HGETALL":
		return pipe.HGetAll(context.Background(), cmd.Key), nil
	case "HDEL":
		return pipe.HDel(context.Background(), cmd.Key, cmd.Field), nil

	// List commands
	case "LPUSH":
		return pipe.LPush(context.Background(), cmd.Key, cmd.Values...), nil
	case "RPUSH":
		return pipe.RPush(context.Background(), cmd.Key, cmd.Values...), nil
	case "LPOP":
		return pipe.LPop(context.Background(), cmd.Key), nil
	case "RPOP":
		return pipe.RPop(context.Background(), cmd.Key), nil
	case "LRANGE":
		// Use default range if not specified
		return pipe.LRange(context.Background(), cmd.Key, 0, -1), nil
	case "LLEN":
		return pipe.LLen(context.Background(), cmd.Key), nil

	// Set commands
	case "SADD":
		return pipe.SAdd(context.Background(), cmd.Key, cmd.Values...), nil
	case "SREM":
		return pipe.SRem(context.Background(), cmd.Key, cmd.Values...), nil
	case "SMEMBERS":
		return pipe.SMembers(context.Background(), cmd.Key), nil
	case "SCARD":
		return pipe.SCard(context.Background(), cmd.Key), nil

	// Sorted set commands
	case "ZADD":
		return pipe.ZAdd(context.Background(), cmd.Key, goredis.Z{Score: cmd.Score, Member: cmd.Value}), nil
	case "ZREM":
		return pipe.ZRem(context.Background(), cmd.Key, cmd.Values...), nil
	case "ZRANGE":
		return pipe.ZRange(context.Background(), cmd.Key, 0, -1), nil
	case "ZCARD":
		return pipe.ZCard(context.Background(), cmd.Key), nil

	// Pub/Sub
	case "PUBLISH":
		channel := cmd.Key // Use key as channel for simplicity
		return pipe.Publish(context.Background(), channel, cmd.Value), nil

	default:
		return nil, fmt.Errorf("unsupported pipeline command: %s", cmd.Command)
	}
}

// extractCmdResult extracts the result from a Redis command.
func extractCmdResult(cmd goredis.Cmder) (any, error) {
	switch c := cmd.(type) {
	case *goredis.StringCmd:
		return c.Result()
	case *goredis.IntCmd:
		return c.Result()
	case *goredis.FloatCmd:
		return c.Result()
	case *goredis.BoolCmd:
		return c.Result()
	case *goredis.StatusCmd:
		return c.Result()
	case *goredis.SliceCmd:
		return c.Result()
	case *goredis.StringSliceCmd:
		return c.Result()
	case *goredis.MapStringStringCmd:
		return c.Result()
	case *goredis.DurationCmd:
		return c.Result()
	case *goredis.ZSliceCmd:
		return c.Result()
	default:
		// For unknown types, return the raw error
		return nil, cmd.Err()
	}
}
