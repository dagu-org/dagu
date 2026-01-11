// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package oidcprovision

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dagu-org/dagu/internal/auth"
	"github.com/itchyny/gojq"
)

// RoleMapperConfig holds configuration for role mapping.
type RoleMapperConfig struct {
	// GroupsClaim specifies the claim name containing groups (default: "groups")
	GroupsClaim string
	// GroupMappings maps IdP group names to Dagu roles
	GroupMappings map[string]string
	// RoleAttributePath is a jq expression to extract role from claims
	RoleAttributePath string
	// RoleAttributeStrict denies login when no valid role is found
	RoleAttributeStrict bool
	// SkipOrgRoleSync skips role sync on subsequent logins
	SkipOrgRoleSync bool
	// DefaultRole is the fallback role when no mapping matches
	DefaultRole auth.Role
}

// RoleMapper maps OIDC claims to Dagu roles.
type RoleMapper struct {
	config  RoleMapperConfig
	jqQuery *gojq.Code // Pre-compiled jq query for performance
}

// ErrNoRoleFound is returned when role mapping fails and strict mode is enabled.
var ErrNoRoleFound = errors.New("no valid role found from OIDC claims")

// NewRoleMapper creates a new RoleMapper with the given configuration.
func NewRoleMapper(config RoleMapperConfig) (*RoleMapper, error) {
	rm := &RoleMapper{config: config}

	// Pre-compile jq query if provided
	if config.RoleAttributePath != "" {
		query, err := gojq.Parse(config.RoleAttributePath)
		if err != nil {
			return nil, fmt.Errorf("invalid roleAttributePath jq expression: %w", err)
		}
		code, err := gojq.Compile(query)
		if err != nil {
			return nil, fmt.Errorf("failed to compile roleAttributePath jq expression: %w", err)
		}
		rm.jqQuery = code
	}

	return rm, nil
}

// MapRole determines the Dagu role from OIDC claims.
// Evaluation order:
//  1. RoleAttributePath (jq expression) if configured
//  2. GroupMappings if configured
//  3. DefaultRole as fallback (or error if strict mode)
func (rm *RoleMapper) MapRole(rawClaims map[string]any) (auth.Role, error) {
	var role auth.Role
	var found bool

	// 1. Try jq expression first (most flexible)
	if rm.jqQuery != nil {
		role, found = rm.evaluateJqExpression(rawClaims)
	}

	// 2. Try group mappings
	if !found && len(rm.config.GroupMappings) > 0 {
		role, found = rm.evaluateGroupMappings(rawClaims)
	}

	// 3. Fallback to default role
	if !found {
		if rm.config.RoleAttributeStrict {
			return "", ErrNoRoleFound
		}
		return rm.config.DefaultRole, nil
	}

	return role, nil
}

// evaluateJqExpression runs the jq query against claims and returns the role.
func (rm *RoleMapper) evaluateJqExpression(claims map[string]any) (auth.Role, bool) {
	iter := rm.jqQuery.Run(claims)
	v, ok := iter.Next()
	if !ok {
		return "", false
	}
	if _, isErr := v.(error); isErr {
		// jq evaluation error - treat as not found
		return "", false
	}

	roleStr, ok := v.(string)
	if !ok || roleStr == "" {
		return "", false
	}

	role := auth.Role(strings.ToLower(roleStr))
	if !role.Valid() {
		return "", false
	}

	return role, true
}

// evaluateGroupMappings checks the groups claim and maps to a role.
// Returns the highest-privilege matching role.
func (rm *RoleMapper) evaluateGroupMappings(claims map[string]any) (auth.Role, bool) {
	groups := rm.extractGroups(claims)
	if len(groups) == 0 {
		return "", false
	}

	// Role priority: admin > manager > operator > viewer
	rolePriority := map[auth.Role]int{
		auth.RoleAdmin:    4,
		auth.RoleManager:  3,
		auth.RoleOperator: 2,
		auth.RoleViewer:   1,
	}

	var bestRole auth.Role
	var bestPriority int

	for _, group := range groups {
		if roleStr, ok := rm.config.GroupMappings[group]; ok {
			role := auth.Role(strings.ToLower(roleStr))
			if role.Valid() {
				priority := rolePriority[role]
				if priority > bestPriority {
					bestRole = role
					bestPriority = priority
				}
			}
		}
	}

	if bestPriority == 0 {
		return "", false
	}

	return bestRole, true
}

// extractGroups extracts group names from claims using the configured claim name.
// Supports nested claims using dot notation (e.g., "realm_access.roles").
func (rm *RoleMapper) extractGroups(claims map[string]any) []string {
	claimName := rm.config.GroupsClaim
	if claimName == "" {
		claimName = "groups" // Default
	}

	// Handle nested claims (e.g., "realm_access.roles" for Keycloak)
	value := getNestedClaim(claims, claimName)
	if value == nil {
		return nil
	}

	// Handle different formats
	switch v := value.(type) {
	case []any:
		groups := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				groups = append(groups, s)
			}
		}
		return groups
	case []string:
		return v
	case string:
		// Some providers send space-separated groups
		return strings.Fields(v)
	}

	return nil
}

// getNestedClaim retrieves a claim value using dot notation.
func getNestedClaim(claims map[string]any, path string) any {
	parts := strings.Split(path, ".")
	current := any(claims)

	for _, part := range parts {
		if m, ok := current.(map[string]any); ok {
			current = m[part]
		} else {
			return nil
		}
	}

	return current
}

// IsConfigured returns true if any role mapping is configured.
func (rm *RoleMapper) IsConfigured() bool {
	return rm.jqQuery != nil || len(rm.config.GroupMappings) > 0
}
