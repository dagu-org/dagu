// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dagucloud/dagu/v2/internal/auth"
	"github.com/dagucloud/dagu/v2/internal/cmn/crypto"
	"github.com/dagucloud/dagu/v2/internal/persis"
)

var _ auth.WebhookStore = (*WebhookStore)(nil)

// webhookLastUsedWriteInterval limits how often a busy webhook rewrites its
// record just to record use.
const webhookLastUsedWriteInterval = time.Minute

// webhookMissRebuildInterval bounds how often lookups of DAGs without a
// webhook reread the whole collection, so a burst of requests for unknown
// names costs one reread, not one each.
const webhookMissRebuildInterval = time.Second

// WebhookStore implements [auth.WebhookStore].
// DAG-name lookups use an in-memory index (byDAGName) built from the
// collection on startup, refreshed by every complete listing and every
// create, and rebuilt when a lookup misses, at most once per
// webhookMissRebuildInterval, since another process sharing the collection
// may have written since; this process's writes keep it in sync under mu.
type WebhookStore struct {
	col       persis.Collection
	encryptor *crypto.Encryptor // nil = HMAC encryption disabled

	mu        sync.RWMutex
	byDAGName map[string]string // dagName → webhookID

	missMu        sync.Mutex
	missRebuiltAt time.Time // guarded by missMu
}

// NewWebhookStore creates a WebhookStore backed by col. enc may be nil when HMAC secrets are unused.
func NewWebhookStore(col persis.Collection, enc *crypto.Encryptor) (*WebhookStore, error) {
	s := &WebhookStore{
		col:       col,
		encryptor: enc,
		byDAGName: make(map[string]string),
	}
	if err := s.rebuildIndex(context.Background()); err != nil {
		return nil, fmt.Errorf("webhook store: build index: %w", err)
	}
	return s, nil
}

// rebuildIndex replaces the index with one read from the collection. It holds
// mu throughout, so a write from this process lands either before the read or
// after the replacement, never lost in between.
func (s *WebhookStore) rebuildIndex(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadIndexLocked(ctx)
}

// loadIndexLocked replaces the index with one read from the collection. The
// caller holds mu.
func (s *WebhookStore) loadIndexLocked(ctx context.Context) error {
	recs, err := listAll(ctx, s.col, persis.ListQuery{})
	if err != nil {
		return err
	}
	index := make(map[string]string, len(recs))
	for _, rec := range recs {
		var stored auth.WebhookForStorage
		if err := persis.Decode(rec, &stored); err != nil {
			continue // skip corrupt records
		}
		index[stored.DAGName] = stored.ID
	}
	s.byDAGName = index
	return nil
}

// idOf returns the ID of dagName's webhook, rebuilding the index when it has
// none, since another process sharing the collection may have created one.
func (s *WebhookStore) idOf(ctx context.Context, dagName string) (string, bool, error) {
	s.mu.RLock()
	id, ok := s.byDAGName[dagName]
	s.mu.RUnlock()
	if ok {
		return id, true, nil
	}
	if err := s.rebuildAfterMiss(ctx); err != nil {
		return "", false, fmt.Errorf("webhook store: rebuild index: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok = s.byDAGName[dagName]
	return id, ok, nil
}

// rebuildAfterMiss rebuilds the index unless a miss rebuilt it within
// webhookMissRebuildInterval. Concurrent misses share one rebuild.
func (s *WebhookStore) rebuildAfterMiss(ctx context.Context) error {
	s.missMu.Lock()
	defer s.missMu.Unlock()
	if time.Since(s.missRebuiltAt) < webhookMissRebuildInterval {
		return nil
	}
	if err := s.rebuildIndex(ctx); err != nil {
		return err
	}
	s.missRebuiltAt = time.Now()
	return nil
}

// Create stores a new webhook.
// Returns [auth.ErrWebhookAlreadyExists] if a webhook for the DAG already exists.
func (s *WebhookStore) Create(ctx context.Context, webhook *auth.Webhook) error {
	if webhook == nil {
		return errors.New("webhook store: webhook cannot be nil")
	}
	if webhook.ID == "" {
		return auth.ErrInvalidWebhookID
	}
	if webhook.DAGName == "" {
		return auth.ErrInvalidWebhookDAGName
	}

	stored, err := s.toStorage(webhook)
	if err != nil {
		return err
	}
	data, err := persis.Encode(stored)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// The collection is read afresh, however lookups have been throttled,
	// since another process may have just created this DAG's webhook.
	if err := s.loadIndexLocked(ctx); err != nil {
		return fmt.Errorf("webhook store: rebuild index: %w", err)
	}
	if _, exists := s.byDAGName[webhook.DAGName]; exists {
		return auth.ErrWebhookAlreadyExists
	}
	if err := s.col.Put(ctx, &persis.Record{
		ID:        webhook.ID,
		Data:      data,
		CreatedAt: webhook.CreatedAt,
		UpdatedAt: webhook.UpdatedAt,
	}); err != nil {
		return err
	}
	s.byDAGName[webhook.DAGName] = webhook.ID
	return nil
}

// GetByID retrieves a webhook by its unique ID.
// Returns [auth.ErrWebhookNotFound] if the webhook does not exist.
func (s *WebhookStore) GetByID(ctx context.Context, id string) (*auth.Webhook, error) {
	if id == "" {
		return nil, auth.ErrInvalidWebhookID
	}
	rec, err := s.col.Get(ctx, id)
	if err != nil {
		if errors.Is(err, persis.ErrNotFound) {
			return nil, auth.ErrWebhookNotFound
		}
		return nil, err
	}
	return s.fromRecord(rec)
}

// GetByDAGName retrieves the webhook for a specific DAG.
// Returns [auth.ErrWebhookNotFound] if no webhook exists for the DAG.
func (s *WebhookStore) GetByDAGName(ctx context.Context, dagName string) (*auth.Webhook, error) {
	if dagName == "" {
		return nil, auth.ErrInvalidWebhookDAGName
	}
	id, ok, err := s.idOf(ctx, dagName)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, auth.ErrWebhookNotFound
	}
	return s.GetByID(ctx, id)
}

// List returns all webhooks in the store.
func (s *WebhookStore) List(ctx context.Context) ([]*auth.Webhook, error) {
	recs, err := listAll(ctx, s.col, persis.ListQuery{})
	if err != nil {
		return nil, err
	}
	out := make([]*auth.Webhook, 0, len(recs))
	for _, rec := range recs {
		wh, err := s.fromRecord(rec)
		if err != nil {
			continue // skip corrupt records
		}
		out = append(out, wh)
	}
	return out, nil
}

// ListComplete returns all webhooks in the store, or an error when any record
// cannot be read, for a caller that must not act on part of the set. It also
// refreshes the DAG-name index, so each webhook it returns can be looked up.
func (s *WebhookStore) ListComplete(ctx context.Context) ([]*auth.Webhook, error) {
	// Held throughout, as in rebuildIndex.
	s.mu.Lock()
	defer s.mu.Unlock()
	recs, err := listAllStrict(ctx, s.col, persis.ListQuery{})
	if err != nil {
		return nil, err
	}
	out := make([]*auth.Webhook, 0, len(recs))
	index := make(map[string]string, len(recs))
	for _, rec := range recs {
		wh, err := s.fromRecord(rec)
		if err != nil {
			return nil, err
		}
		out = append(out, wh)
		index[wh.DAGName] = wh.ID
	}
	s.byDAGName = index
	return out, nil
}

// UpdateByDAGName applies mutate to the current webhook for dagName and stores
// the result. Concurrent updates, including those from other processes sharing
// the collection, never overwrite each other. mutate may run more than once.
// An error from mutate aborts the update and is returned unchanged.
// Returns [auth.ErrWebhookNotFound] if no webhook exists for the DAG.
func (s *WebhookStore) UpdateByDAGName(
	ctx context.Context,
	dagName string,
	mutate func(*auth.Webhook) error,
) (*auth.Webhook, error) {
	if dagName == "" {
		return nil, auth.ErrInvalidWebhookDAGName
	}

	id, ok, err := s.idOf(ctx, dagName)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, auth.ErrWebhookNotFound
	}

	var updated *auth.Webhook
	err = retryConflict(ctx, func(ctx context.Context) error {
		rec, err := s.col.Get(ctx, id)
		if err != nil {
			return err
		}
		webhook, err := s.fromRecord(rec)
		if err != nil {
			return err
		}
		if err := mutate(webhook); err != nil {
			return err
		}
		if webhook.ID != id || webhook.DAGName != dagName {
			return errors.New("webhook store: update cannot change the webhook ID or DAG name")
		}
		stored, err := s.toStorage(webhook)
		if err != nil {
			return err
		}
		data, err := persis.Encode(stored)
		if err != nil {
			return err
		}
		if err := s.col.CompareAndSwap(ctx, id, rec.Data, data); err != nil {
			return err
		}
		updated = webhook
		return nil
	})
	if errors.Is(err, persis.ErrNotFound) {
		return nil, auth.ErrWebhookNotFound
	}
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// RenameDAG moves a DAG's webhook to the DAG's new name, keeping its tokens
// and settings. Returns [auth.ErrWebhookNotFound] if no webhook exists for
// oldName, and [auth.ErrWebhookAlreadyExists] if one exists for newName.
func (s *WebhookStore) RenameDAG(ctx context.Context, oldName, newName string) error {
	if oldName == "" || newName == "" {
		return auth.ErrInvalidWebhookDAGName
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Read afresh, as for a create, since the new name must be free.
	if err := s.loadIndexLocked(ctx); err != nil {
		return fmt.Errorf("webhook store: rebuild index: %w", err)
	}
	id, ok := s.byDAGName[oldName]
	if !ok {
		return auth.ErrWebhookNotFound
	}
	if _, exists := s.byDAGName[newName]; exists {
		return auth.ErrWebhookAlreadyExists
	}
	err := retryConflict(ctx, func(ctx context.Context) error {
		rec, err := s.col.Get(ctx, id)
		if err != nil {
			return err
		}
		// The stored form is moved as it is, so the encrypted secrets need
		// no encryptor.
		var stored auth.WebhookForStorage
		if err := persis.Decode(rec, &stored); err != nil {
			return err
		}
		stored.DAGName = newName
		stored.UpdatedAt = time.Now().UTC()
		data, err := persis.Encode(stored)
		if err != nil {
			return err
		}
		return s.col.CompareAndSwap(ctx, id, rec.Data, data)
	})
	if errors.Is(err, persis.ErrNotFound) {
		return auth.ErrWebhookNotFound
	}
	if err != nil {
		return err
	}
	delete(s.byDAGName, oldName)
	s.byDAGName[newName] = id
	return nil
}

// Delete removes a webhook by its ID.
// Returns [auth.ErrWebhookNotFound] if the webhook does not exist.
func (s *WebhookStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return auth.ErrInvalidWebhookID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	rec, err := s.col.Get(ctx, id)
	if err != nil {
		if errors.Is(err, persis.ErrNotFound) {
			return auth.ErrWebhookNotFound
		}
		return err
	}
	var stored auth.WebhookForStorage
	if err := persis.Decode(rec, &stored); err != nil {
		return fmt.Errorf("webhook store: decode for delete: %w", err)
	}

	if err := s.col.Delete(ctx, id); err != nil {
		return err
	}
	delete(s.byDAGName, stored.DAGName)
	return nil
}

// DeleteByDAGName removes a webhook by its DAG name.
// Returns [auth.ErrWebhookNotFound] if no webhook exists for the DAG.
func (s *WebhookStore) DeleteByDAGName(ctx context.Context, dagName string) error {
	if dagName == "" {
		return auth.ErrInvalidWebhookDAGName
	}
	id, ok, err := s.idOf(ctx, dagName)
	if err != nil {
		return err
	}
	if !ok {
		return auth.ErrWebhookNotFound
	}
	return s.Delete(ctx, id)
}

// UpdateLastUsed records recent use of a webhook and, when profileTokenID is
// not empty, of that profile token, persisting each timestamp at most once per
// minute. It never restores a profile token removed by a concurrent update.
func (s *WebhookStore) UpdateLastUsed(ctx context.Context, id, profileTokenID string) error {
	if id == "" {
		return auth.ErrInvalidWebhookID
	}
	err := retryConflict(ctx, func(ctx context.Context) error {
		rec, err := s.col.Get(ctx, id)
		if err != nil {
			return err
		}
		var stored auth.WebhookForStorage
		if err := persis.Decode(rec, &stored); err != nil {
			return fmt.Errorf("webhook store: decode for UpdateLastUsed: %w", err)
		}
		now := time.Now().UTC()
		changed := false
		if lastUsedStale(stored.LastUsedAt, now) {
			stored.LastUsedAt = &now
			changed = true
		}
		for i := range stored.ProfileTokens {
			token := &stored.ProfileTokens[i]
			if token.ID == profileTokenID && lastUsedStale(token.LastUsedAt, now) {
				token.LastUsedAt = &now
				changed = true
			}
		}
		if !changed {
			return nil
		}
		data, err := persis.Encode(stored)
		if err != nil {
			return err
		}
		return s.col.CompareAndSwap(ctx, id, rec.Data, data)
	})
	if errors.Is(err, persis.ErrNotFound) {
		return auth.ErrWebhookNotFound
	}
	return err
}

func lastUsedStale(lastUsed *time.Time, now time.Time) bool {
	return lastUsed == nil || now.Sub(*lastUsed) >= webhookLastUsedWriteInterval
}

// ─── encoding helpers ─────────────────────────────────────────────────────────

func (s *WebhookStore) toStorage(wh *auth.Webhook) (*auth.WebhookForStorage, error) {
	stored := wh.ToStorage()
	if wh.HMACSecret != "" {
		if s.encryptor == nil {
			return nil, auth.ErrWebhookHMACEncryptorRequired
		}
		enc, err := s.encryptor.Encrypt(wh.HMACSecret)
		if err != nil {
			return nil, fmt.Errorf("webhook store: encrypt HMAC secret: %w", err)
		}
		stored.HMACSecretEnc = enc
	}
	if wh.Public != nil {
		if s.encryptor == nil {
			return nil, auth.ErrWebhookPublicEncryptorRequired
		}
		enc, err := s.encryptor.Encrypt(wh.Public.Token)
		if err != nil {
			return nil, fmt.Errorf("webhook store: encrypt public URL token: %w", err)
		}
		stored.Public.TokenEnc = enc
		if wh.Public.SenderSecret != "" {
			enc, err := s.encryptor.Encrypt(wh.Public.SenderSecret)
			if err != nil {
				return nil, fmt.Errorf("webhook store: encrypt sender secret: %w", err)
			}
			stored.Public.SenderSecretEnc = enc
		}
	}
	return stored, nil
}

func (s *WebhookStore) fromRecord(rec *persis.Record) (*auth.Webhook, error) {
	var stored auth.WebhookForStorage
	if err := persis.Decode(rec, &stored); err != nil {
		return nil, fmt.Errorf("webhook store: decode record %q: %w", rec.ID, err)
	}
	wh := stored.ToWebhook()
	if stored.HMACSecretEnc != "" {
		if s.encryptor == nil {
			return nil, auth.ErrWebhookHMACEncryptorRequired
		}
		secret, err := s.encryptor.Decrypt(stored.HMACSecretEnc)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", auth.ErrWebhookHMACDecryptFailed, err)
		}
		wh.HMACSecret = secret
	}
	if stored.Public != nil {
		if s.encryptor == nil {
			return nil, auth.ErrWebhookPublicEncryptorRequired
		}
		token, err := s.encryptor.Decrypt(stored.Public.TokenEnc)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", auth.ErrWebhookPublicDecryptFailed, err)
		}
		wh.Public.Token = token
		if stored.Public.SenderSecretEnc != "" {
			secret, err := s.encryptor.Decrypt(stored.Public.SenderSecretEnc)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", auth.ErrWebhookPublicDecryptFailed, err)
			}
			wh.Public.SenderSecret = secret
		}
	}
	return wh, nil
}
