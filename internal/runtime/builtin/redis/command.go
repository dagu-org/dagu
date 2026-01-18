package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// nilToNull converts redis.Nil errors to nil results.
// Returns (result, nil) for non-nil results, (nil, nil) for redis.Nil, or (nil, err) for other errors.
func nilToNull[T any](result T, err error) (any, error) {
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

// CommandHandler executes Redis commands.
type CommandHandler struct {
	client redis.UniversalClient
	cfg    *Config
}

// NewCommandHandler creates a new command handler.
func NewCommandHandler(client redis.UniversalClient, cfg *Config) *CommandHandler {
	return &CommandHandler{client: client, cfg: cfg}
}

// Execute executes the configured Redis command and returns the result.
func (h *CommandHandler) Execute(ctx context.Context) (any, error) {
	switch strings.ToUpper(h.cfg.Command) {
	// String commands
	case "GET":
		return nilToNull(h.client.Get(ctx, h.cfg.Key).Result())

	case "SET":
		ttl := time.Duration(h.cfg.TTL) * time.Second
		// Handle KeepTTL option
		if h.cfg.KeepTTL {
			ttl = redis.KeepTTL
		}
		if h.cfg.NX {
			return h.client.SetNX(ctx, h.cfg.Key, h.cfg.Value, ttl).Result()
		}
		if h.cfg.XX {
			return h.client.SetXX(ctx, h.cfg.Key, h.cfg.Value, ttl).Result()
		}
		return h.client.Set(ctx, h.cfg.Key, h.cfg.Value, ttl).Result()

	case "SETNX":
		ttl := time.Duration(h.cfg.TTL) * time.Second
		return h.client.SetNX(ctx, h.cfg.Key, h.cfg.Value, ttl).Result()

	case "SETEX":
		ttl := time.Duration(h.cfg.TTL) * time.Second
		return h.client.SetEx(ctx, h.cfg.Key, h.cfg.Value, ttl).Result()

	case "GETSET":
		return h.client.GetSet(ctx, h.cfg.Key, h.cfg.Value).Result()

	case "MGET":
		return h.client.MGet(ctx, h.cfg.Keys...).Result()

	case "MSET":
		return h.client.MSet(ctx, h.cfg.Values...).Result()

	case "INCR":
		return h.client.Incr(ctx, h.cfg.Key).Result()

	case "INCRBY":
		delta, err := toInt64(h.cfg.Value)
		if err != nil {
			return nil, fmt.Errorf("incrby requires integer value: %w", err)
		}
		return h.client.IncrBy(ctx, h.cfg.Key, delta).Result()

	case "INCRBYFLOAT":
		delta, err := toFloat64(h.cfg.Value)
		if err != nil {
			return nil, fmt.Errorf("incrbyfloat requires numeric value: %w", err)
		}
		return h.client.IncrByFloat(ctx, h.cfg.Key, delta).Result()

	case "DECR":
		return h.client.Decr(ctx, h.cfg.Key).Result()

	case "DECRBY":
		delta, err := toInt64(h.cfg.Value)
		if err != nil {
			return nil, fmt.Errorf("decrby requires integer value: %w", err)
		}
		return h.client.DecrBy(ctx, h.cfg.Key, delta).Result()

	case "APPEND":
		return h.client.Append(ctx, h.cfg.Key, toString(h.cfg.Value)).Result()

	case "STRLEN":
		return h.client.StrLen(ctx, h.cfg.Key).Result()

	// Key commands
	case "DEL":
		keys := h.cfg.Keys
		if len(keys) == 0 && h.cfg.Key != "" {
			keys = []string{h.cfg.Key}
		}
		return h.client.Del(ctx, keys...).Result()

	case "EXISTS":
		keys := h.cfg.Keys
		if len(keys) == 0 && h.cfg.Key != "" {
			keys = []string{h.cfg.Key}
		}
		return h.client.Exists(ctx, keys...).Result()

	case "EXPIRE":
		return h.client.Expire(ctx, h.cfg.Key, time.Duration(h.cfg.TTL)*time.Second).Result()

	case "EXPIREAT":
		ts, err := toInt64(h.cfg.Value)
		if err != nil {
			return nil, fmt.Errorf("expireat requires unix timestamp: %w", err)
		}
		return h.client.ExpireAt(ctx, h.cfg.Key, time.Unix(ts, 0)).Result()

	case "TTL":
		return h.client.TTL(ctx, h.cfg.Key).Result()

	case "PTTL":
		return h.client.PTTL(ctx, h.cfg.Key).Result()

	case "PERSIST":
		return h.client.Persist(ctx, h.cfg.Key).Result()

	case "KEYS":
		pattern := h.cfg.Match
		if pattern == "" {
			pattern = "*"
		}
		return h.client.Keys(ctx, pattern).Result()

	case "SCAN":
		return h.executeScan(ctx)

	case "TYPE":
		return h.client.Type(ctx, h.cfg.Key).Result()

	case "RENAME":
		if len(h.cfg.Keys) < 1 {
			return nil, fmt.Errorf("rename requires key and newkey")
		}
		return h.client.Rename(ctx, h.cfg.Key, h.cfg.Keys[0]).Result()

	case "RENAMENX":
		if len(h.cfg.Keys) < 1 {
			return nil, fmt.Errorf("renamenx requires key and newkey")
		}
		return h.client.RenameNX(ctx, h.cfg.Key, h.cfg.Keys[0]).Result()

	// Hash commands
	case "HGET":
		return nilToNull(h.client.HGet(ctx, h.cfg.Key, h.cfg.Field).Result())

	case "HSET":
		return h.client.HSet(ctx, h.cfg.Key, h.flattenFields()...).Result()

	case "HSETNX":
		return h.client.HSetNX(ctx, h.cfg.Key, h.cfg.Field, h.cfg.Value).Result()

	case "HMGET":
		fields := h.cfg.Keys // Reuse keys for field names
		if len(fields) == 0 && h.cfg.Field != "" {
			fields = []string{h.cfg.Field}
		}
		return h.client.HMGet(ctx, h.cfg.Key, fields...).Result()

	case "HMSET":
		return h.client.HMSet(ctx, h.cfg.Key, h.cfg.Fields).Result()

	case "HGETALL":
		return h.client.HGetAll(ctx, h.cfg.Key).Result()

	case "HDEL":
		fields := h.cfg.Keys // Reuse keys for field names
		if len(fields) == 0 && h.cfg.Field != "" {
			fields = []string{h.cfg.Field}
		}
		return h.client.HDel(ctx, h.cfg.Key, fields...).Result()

	case "HEXISTS":
		return h.client.HExists(ctx, h.cfg.Key, h.cfg.Field).Result()

	case "HINCRBY":
		delta, err := toInt64(h.cfg.Value)
		if err != nil {
			return nil, fmt.Errorf("hincrby requires integer value: %w", err)
		}
		return h.client.HIncrBy(ctx, h.cfg.Key, h.cfg.Field, delta).Result()

	case "HINCRBYFLOAT":
		delta, err := toFloat64(h.cfg.Value)
		if err != nil {
			return nil, fmt.Errorf("hincrbyfloat requires numeric value: %w", err)
		}
		return h.client.HIncrByFloat(ctx, h.cfg.Key, h.cfg.Field, delta).Result()

	case "HKEYS":
		return h.client.HKeys(ctx, h.cfg.Key).Result()

	case "HVALS":
		return h.client.HVals(ctx, h.cfg.Key).Result()

	case "HLEN":
		return h.client.HLen(ctx, h.cfg.Key).Result()

	// List commands
	case "LPUSH":
		return h.client.LPush(ctx, h.cfg.Key, h.cfg.Values...).Result()

	case "RPUSH":
		return h.client.RPush(ctx, h.cfg.Key, h.cfg.Values...).Result()

	case "LPOP":
		return nilToNull(h.client.LPop(ctx, h.cfg.Key).Result())

	case "RPOP":
		return nilToNull(h.client.RPop(ctx, h.cfg.Key).Result())

	case "BLPOP":
		timeout := time.Duration(h.cfg.Block) * time.Millisecond
		if timeout == 0 {
			timeout = time.Duration(h.cfg.Timeout) * time.Second
		}
		return nilToNull(h.client.BLPop(ctx, timeout, h.cfg.Key).Result())

	case "BRPOP":
		timeout := time.Duration(h.cfg.Block) * time.Millisecond
		if timeout == 0 {
			timeout = time.Duration(h.cfg.Timeout) * time.Second
		}
		return nilToNull(h.client.BRPop(ctx, timeout, h.cfg.Key).Result())

	case "LRANGE":
		return h.client.LRange(ctx, h.cfg.Key, h.cfg.Start, h.cfg.Stop).Result()

	case "LLEN":
		return h.client.LLen(ctx, h.cfg.Key).Result()

	case "LINDEX":
		index, err := toInt64(h.cfg.Value)
		if err != nil {
			return nil, fmt.Errorf("lindex requires integer index: %w", err)
		}
		return nilToNull(h.client.LIndex(ctx, h.cfg.Key, index).Result())

	case "LSET":
		return h.client.LSet(ctx, h.cfg.Key, h.cfg.Start, h.cfg.Value).Result()

	case "LINSERT":
		if h.cfg.Position == "" || h.cfg.Pivot == "" {
			return nil, fmt.Errorf("linsert requires position and pivot")
		}
		return h.client.LInsert(ctx, h.cfg.Key, h.cfg.Position, h.cfg.Pivot, h.cfg.Value).Result()

	case "LREM":
		count, err := toInt64(h.cfg.Value)
		if err != nil {
			count = 0
		}
		// Use second value or field for element
		element := h.cfg.Field
		if len(h.cfg.Values) > 0 {
			element = toString(h.cfg.Values[0])
		}
		return h.client.LRem(ctx, h.cfg.Key, count, element).Result()

	case "LTRIM":
		return h.client.LTrim(ctx, h.cfg.Key, h.cfg.Start, h.cfg.Stop).Result()

	case "LMOVE":
		if len(h.cfg.Keys) < 1 {
			return nil, fmt.Errorf("lmove requires source and destination keys")
		}
		srcDir := "LEFT"
		dstDir := "LEFT"
		if h.cfg.Position != "" {
			srcDir = h.cfg.Position
		}
		if dest, ok := h.cfg.Fields["destPosition"].(string); ok {
			dstDir = dest
		}
		return h.client.LMove(ctx, h.cfg.Key, h.cfg.Keys[0], srcDir, dstDir).Result()

	// Set commands
	case "SADD":
		return h.client.SAdd(ctx, h.cfg.Key, h.cfg.Values...).Result()

	case "SREM":
		return h.client.SRem(ctx, h.cfg.Key, h.cfg.Values...).Result()

	case "SMEMBERS":
		return h.client.SMembers(ctx, h.cfg.Key).Result()

	case "SISMEMBER":
		return h.client.SIsMember(ctx, h.cfg.Key, h.cfg.Value).Result()

	case "SCARD":
		return h.client.SCard(ctx, h.cfg.Key).Result()

	case "SPOP":
		return nilToNull(h.client.SPop(ctx, h.cfg.Key).Result())

	case "SRANDMEMBER":
		count := int64(h.cfg.Count)
		if count == 0 {
			count = 1
		}
		return h.client.SRandMemberN(ctx, h.cfg.Key, count).Result()

	case "SDIFF":
		return h.client.SDiff(ctx, h.cfg.Keys...).Result()

	case "SINTER":
		return h.client.SInter(ctx, h.cfg.Keys...).Result()

	case "SUNION":
		return h.client.SUnion(ctx, h.cfg.Keys...).Result()

	// Sorted set commands
	case "ZADD":
		return h.client.ZAdd(ctx, h.cfg.Key, redis.Z{Score: h.cfg.Score, Member: h.cfg.Value}).Result()

	case "ZREM":
		return h.client.ZRem(ctx, h.cfg.Key, h.cfg.Values...).Result()

	case "ZRANGE":
		if h.cfg.WithScores {
			return h.client.ZRangeWithScores(ctx, h.cfg.Key, h.cfg.Start, h.cfg.Stop).Result()
		}
		return h.client.ZRange(ctx, h.cfg.Key, h.cfg.Start, h.cfg.Stop).Result()

	case "ZREVRANGE":
		if h.cfg.WithScores {
			return h.client.ZRevRangeWithScores(ctx, h.cfg.Key, h.cfg.Start, h.cfg.Stop).Result()
		}
		return h.client.ZRevRange(ctx, h.cfg.Key, h.cfg.Start, h.cfg.Stop).Result()

	case "ZRANGEBYSCORE":
		opt := &redis.ZRangeBy{
			Min: h.cfg.Min,
			Max: h.cfg.Max,
		}
		if h.cfg.Count > 0 {
			opt.Count = int64(h.cfg.Count)
		}
		if h.cfg.WithScores {
			return h.client.ZRangeByScoreWithScores(ctx, h.cfg.Key, opt).Result()
		}
		return h.client.ZRangeByScore(ctx, h.cfg.Key, opt).Result()

	case "ZSCORE":
		return nilToNull(h.client.ZScore(ctx, h.cfg.Key, toString(h.cfg.Value)).Result())

	case "ZRANK":
		return nilToNull(h.client.ZRank(ctx, h.cfg.Key, toString(h.cfg.Value)).Result())

	case "ZREVRANK":
		return nilToNull(h.client.ZRevRank(ctx, h.cfg.Key, toString(h.cfg.Value)).Result())

	case "ZCARD":
		return h.client.ZCard(ctx, h.cfg.Key).Result()

	case "ZCOUNT":
		return h.client.ZCount(ctx, h.cfg.Key, h.cfg.Min, h.cfg.Max).Result()

	case "ZINCRBY":
		return h.client.ZIncrBy(ctx, h.cfg.Key, h.cfg.Score, toString(h.cfg.Value)).Result()

	case "ZPOPMIN":
		count := int64(h.cfg.Count)
		if count == 0 {
			count = 1
		}
		return h.client.ZPopMin(ctx, h.cfg.Key, count).Result()

	case "ZPOPMAX":
		count := int64(h.cfg.Count)
		if count == 0 {
			count = 1
		}
		return h.client.ZPopMax(ctx, h.cfg.Key, count).Result()

	// Pub/Sub
	case "PUBLISH":
		return h.client.Publish(ctx, h.cfg.Channel, h.cfg.Message).Result()

	// Stream commands
	case "XADD":
		args := &redis.XAddArgs{
			Stream: h.cfg.Stream,
			ID:     h.cfg.StreamID,
			Values: h.cfg.StreamFields,
		}
		if h.cfg.MaxLen > 0 {
			args.MaxLen = h.cfg.MaxLen
		}
		if args.ID == "" {
			args.ID = "*"
		}
		return h.client.XAdd(ctx, args).Result()

	case "XREAD":
		return nilToNull(h.client.XRead(ctx, &redis.XReadArgs{
			Streams: []string{h.cfg.Stream, h.cfg.StreamID},
			Count:   int64(h.cfg.Count),
			Block:   time.Duration(h.cfg.Block) * time.Millisecond,
		}).Result())

	case "XREADGROUP":
		return nilToNull(h.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    h.cfg.Group,
			Consumer: h.cfg.Consumer,
			Streams:  []string{h.cfg.Stream, h.cfg.StreamID},
			Count:    int64(h.cfg.Count),
			Block:    time.Duration(h.cfg.Block) * time.Millisecond,
			NoAck:    h.cfg.NoAck,
		}).Result())

	case "XACK":
		ids := h.cfg.Keys // Reuse keys for message IDs
		return h.client.XAck(ctx, h.cfg.Stream, h.cfg.Group, ids...).Result()

	case "XLEN":
		return h.client.XLen(ctx, h.cfg.Stream).Result()

	case "XRANGE":
		min := h.cfg.Min
		max := h.cfg.Max
		if min == "" {
			min = "-"
		}
		if max == "" {
			max = "+"
		}
		return h.client.XRange(ctx, h.cfg.Stream, min, max).Result()

	case "XREVRANGE":
		min := h.cfg.Min
		max := h.cfg.Max
		if min == "" {
			min = "-"
		}
		if max == "" {
			max = "+"
		}
		return h.client.XRevRange(ctx, h.cfg.Stream, max, min).Result()

	case "XGROUP CREATE", "XGROUP_CREATE":
		id := h.cfg.StreamID
		if id == "" {
			id = "$"
		}
		return h.client.XGroupCreate(ctx, h.cfg.Stream, h.cfg.Group, id).Result()

	case "XGROUP DESTROY", "XGROUP_DESTROY":
		return h.client.XGroupDestroy(ctx, h.cfg.Stream, h.cfg.Group).Result()

	case "XGROUP DELCONSUMER", "XGROUP_DELCONSUMER":
		return h.client.XGroupDelConsumer(ctx, h.cfg.Stream, h.cfg.Group, h.cfg.Consumer).Result()

	case "XINFO GROUPS", "XINFO_GROUPS":
		return h.client.XInfoGroups(ctx, h.cfg.Stream).Result()

	case "XINFO STREAM", "XINFO_STREAM":
		return h.client.XInfoStream(ctx, h.cfg.Stream).Result()

	case "XPENDING":
		return h.client.XPending(ctx, h.cfg.Stream, h.cfg.Group).Result()

	// Server commands
	case "PING":
		return h.client.Ping(ctx).Result()

	case "ECHO":
		return h.client.Echo(ctx, toString(h.cfg.Value)).Result()

	case "INFO":
		section := toString(h.cfg.Value)
		if section == "" {
			return h.client.Info(ctx).Result()
		}
		return h.client.Info(ctx, section).Result()

	case "DBSIZE":
		return h.client.DBSize(ctx).Result()

	case "FLUSHDB":
		return h.client.FlushDB(ctx).Result()

	case "FLUSHALL":
		return h.client.FlushAll(ctx).Result()

	case "TIME":
		return h.client.Time(ctx).Result()

	case "CLIENT ID", "CLIENT_ID":
		return h.client.ClientID(ctx).Result()

	case "CLIENT LIST", "CLIENT_LIST":
		return h.client.ClientList(ctx).Result()

	default:
		return nil, fmt.Errorf("unsupported command: %s", h.cfg.Command)
	}
}

// maxScanResults is the maximum number of keys to return from SCAN to prevent memory issues.
const maxScanResults = 100000

// executeScan performs a SCAN operation and collects results with memory safeguards.
func (h *CommandHandler) executeScan(ctx context.Context) ([]string, error) {
	var allKeys []string
	var cursor uint64
	count := int64(h.cfg.Count)
	if count == 0 {
		count = 100
	}
	match := h.cfg.Match
	if match == "" {
		match = "*"
	}

	for {
		keys, nextCursor, err := h.client.Scan(ctx, cursor, match, count).Result()
		if err != nil {
			return nil, err
		}
		allKeys = append(allKeys, keys...)

		// Safeguard: limit total results to prevent memory exhaustion on large datasets
		if len(allKeys) >= maxScanResults {
			return allKeys[:maxScanResults], nil
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return allKeys, nil
}

// flattenFields converts a map of fields to a slice for HSet.
func (h *CommandHandler) flattenFields() []any {
	if h.cfg.Field != "" && h.cfg.Value != nil {
		return []any{h.cfg.Field, h.cfg.Value}
	}
	result := make([]any, 0, len(h.cfg.Fields)*2)
	for k, v := range h.cfg.Fields {
		result = append(result, k, v)
	}
	return result
}

// toInt64 converts a value to int64.
func toInt64(v any) (int64, error) {
	switch val := v.(type) {
	case int:
		return int64(val), nil
	case int32:
		return int64(val), nil
	case int64:
		return val, nil
	case float64:
		return int64(val), nil
	case float32:
		return int64(val), nil
	case string:
		return strconv.ParseInt(val, 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", v)
	}
}

// toFloat64 converts a value to float64.
func toFloat64(v any) (float64, error) {
	switch val := v.(type) {
	case int:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

// toString converts a value to string.
func toString(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", v)
	}
}
