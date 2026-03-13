// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package sse

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTopicCanonicalizesQuery(t *testing.T) {
	parsed, err := ParseTopic("dagslist:perPage=100&page=1")
	require.NoError(t, err)

	assert.Equal(t, TopicTypeDAGsList, parsed.Type)
	assert.Equal(t, "page=1&perPage=100", parsed.Identifier)
	assert.Equal(t, "dagslist:page=1&perPage=100", parsed.Key)
}

func TestParseInitialTopics(t *testing.T) {
	query := map[string][]string{
		"topic":  {"dag:test.yaml", "agent:sess-1"},
		"topics": {"ignored:topic"},
	}

	assert.Equal(t, []string{"dag:test.yaml", "agent:sess-1"}, parseInitialTopics(query))

	query = map[string][]string{
		"topics": {"dag:test.yaml,agent:sess-1"},
	}
	assert.Equal(t, []string{"dag:test.yaml", "agent:sess-1"}, parseInitialTopics(query))
}

func TestMultiplexerCreateSessionFiltersUnauthorizedTopics(t *testing.T) {
	mux := NewMultiplexer(StreamConfig{}, nil)
	t.Cleanup(mux.Shutdown)

	mux.RegisterFetcher(TopicTypeDAG, func(_ context.Context, identifier string) (any, error) {
		return map[string]string{"id": identifier}, nil
	})
	mux.RegisterFetcher(TopicTypeAgent, func(_ context.Context, identifier string) (any, error) {
		return map[string]string{"id": identifier}, nil
	})
	mux.RegisterAuthorizer(TopicTypeAgent, func(_ context.Context, _ string) error {
		return errors.New("forbidden")
	})

	recorder := httptest.NewRecorder()
	session, control, _, err := mux.CreateSession(
		context.Background(),
		recorder,
		[]string{"dag:test.yaml", "agent:sess-1"},
		0,
	)
	require.NoError(t, err)
	require.NotNil(t, session)

	assert.Equal(t, []string{"dag:test.yaml"}, control.Subscribed)
	require.Len(t, control.Errors, 1)
	assert.Equal(t, "agent:sess-1", control.Errors[0].Topic)
}

func TestMultiplexerMutateSessionPartialAuthorization(t *testing.T) {
	mux := NewMultiplexer(StreamConfig{}, nil)
	t.Cleanup(mux.Shutdown)

	mux.RegisterFetcher(TopicTypeDAG, func(_ context.Context, identifier string) (any, error) {
		return map[string]string{"id": identifier}, nil
	})
	mux.RegisterFetcher(TopicTypeAgent, func(_ context.Context, identifier string) (any, error) {
		return map[string]string{"id": identifier}, nil
	})
	mux.RegisterAuthorizer(TopicTypeAgent, func(_ context.Context, _ string) error {
		return errors.New("forbidden")
	})

	recorder := httptest.NewRecorder()
	session, _, _, err := mux.CreateSession(context.Background(), recorder, nil, 0)
	require.NoError(t, err)
	require.NotNil(t, session)

	response, _, statusCode, err := mux.MutateSession(
		context.Background(),
		session.id,
		[]string{"dag:test.yaml", "agent:sess-1"},
		nil,
	)
	require.NoError(t, err)

	assert.Equal(t, 403, statusCode)
	assert.Equal(t, []string{"dag:test.yaml"}, response.Subscribed)
	require.Len(t, response.Errors, 1)
	assert.Equal(t, "agent:sess-1", response.Errors[0].Topic)
}

func TestMultiplexerMutateSessionIsAtomicOnTopicResolutionFailure(t *testing.T) {
	mux := NewMultiplexer(StreamConfig{}, nil)
	t.Cleanup(mux.Shutdown)

	mux.RegisterFetcher(TopicTypeDAG, func(_ context.Context, identifier string) (any, error) {
		return map[string]string{"id": identifier}, nil
	})

	recorder := httptest.NewRecorder()
	session, _, _, err := mux.CreateSession(context.Background(), recorder, []string{"dag:test.yaml"}, 0)
	require.NoError(t, err)
	require.NotNil(t, session)

	_, _, _, err = mux.MutateSession(
		context.Background(),
		session.id,
		[]string{"missing:test"},
		[]string{"dag:test.yaml"},
	)
	require.Error(t, err)

	assert.Equal(t, []string{"dag:test.yaml"}, session.topicKeys())
	_, missingTopicExists := mux.topics["missing:test"]
	assert.False(t, missingTopicExists)
}

func TestMultiplexerMutateSessionRejectsConflictingTopics(t *testing.T) {
	mux := NewMultiplexer(StreamConfig{}, nil)
	t.Cleanup(mux.Shutdown)

	mux.RegisterFetcher(TopicTypeDAG, func(_ context.Context, identifier string) (any, error) {
		return map[string]string{"id": identifier}, nil
	})

	recorder := httptest.NewRecorder()
	session, _, _, err := mux.CreateSession(context.Background(), recorder, []string{"dag:test.yaml"}, 0)
	require.NoError(t, err)
	require.NotNil(t, session)

	_, _, _, err = mux.MutateSession(
		context.Background(),
		session.id,
		[]string{"dag:test.yaml"},
		[]string{"dag:test.yaml"},
	)
	require.ErrorIs(t, err, ErrConflictingTopicMutation)
	assert.Equal(t, []string{"dag:test.yaml"}, session.topicKeys())
}

func TestMultiplexerCreateSessionDoesNotRetainTopicsOnFailure(t *testing.T) {
	mux := NewMultiplexer(StreamConfig{}, nil)
	t.Cleanup(mux.Shutdown)

	mux.RegisterFetcher(TopicTypeDAG, func(_ context.Context, identifier string) (any, error) {
		return map[string]string{"id": identifier}, nil
	})

	recorder := httptest.NewRecorder()
	session, _, _, err := mux.CreateSession(
		context.Background(),
		recorder,
		[]string{"dag:test.yaml", "missing:test"},
		0,
	)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Empty(t, mux.sessions)
	assert.Empty(t, mux.topics)
}

func TestMultiplexerSharesTopicRegistryAcrossSessions(t *testing.T) {
	mux := NewMultiplexer(StreamConfig{}, nil)
	t.Cleanup(mux.Shutdown)

	mux.RegisterFetcher(TopicTypeDAG, func(_ context.Context, identifier string) (any, error) {
		return map[string]string{"id": identifier}, nil
	})

	for range 20 {
		recorder := httptest.NewRecorder()
		session, _, _, err := mux.CreateSession(context.Background(), recorder, []string{"dag:test.yaml"}, 0)
		require.NoError(t, err)
		require.NotNil(t, session)
	}

	require.Len(t, mux.topics, 1)
	assert.Contains(t, mux.topics, "dag:test.yaml")
}

func TestBuildRemoteEventURLStripsSensitiveQueryParams(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/events/stream?topic=dag%3Atest.yaml&remoteNode=remote1&token=secret", nil)

	remoteURL := buildRemoteStreamURL("https://remote.example.com/api/v1", req.URL.Query())

	assert.Equal(t, "https://remote.example.com/api/v1/events/stream?topic=dag%3Atest.yaml", remoteURL)
}

func TestBuildRemoteTopicMutationURLStripsSensitiveQueryParams(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/events/stream/topics?remoteNode=remote1&token=secret", nil)

	remoteURL := buildRemoteTopicMutationURL("https://remote.example.com/api/v1", req.URL.Query())

	assert.Equal(t, "https://remote.example.com/api/v1/events/stream/topics", remoteURL)
}
