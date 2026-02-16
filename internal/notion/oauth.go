// Package notion provides OAuth token management for Notion API.
package notion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/wesm/msgvault/internal/fileutil"
)

// Error types for token operations.
var (
	ErrInvalidToken    = errors.New("invalid or malformed Notion token")
	ErrTokenNotFound   = errors.New("token not found")
	ErrPermissionDenied = errors.New("permission denied")
)

const (
	// notionTokenPrefix is the prefix for valid Notion API tokens.
	notionTokenPrefix = "ntn_"

	// minTokenLength is the minimum length for a valid Notion token.
	minTokenLength = 40
)

// Manager handles Notion OAuth token storage and lifecycle.
type Manager struct {
	tokensDir string
	logger    *slog.Logger
}

// tokenFile wraps a Notion API token with metadata.
type tokenFile struct {
	Token     string `json:"token"`
	Workspace string `json:"workspace"`
}

// NewManager creates a new Notion OAuth token manager.
// It creates tokensDir if it doesn't exist with secure permissions.
func NewManager(tokensDir string, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}

	return &Manager{
		tokensDir: tokensDir,
		logger:    logger,
	}
}

// StoreToken validates and stores a Notion API token for a workspace.
// Token must start with "ntn_" and be at least 40 characters.
// File is written with 0600 (owner read-write only) permissions.
func (m *Manager) StoreToken(ctx context.Context, workspace string, token string) error {
	// Context check
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Validate inputs
	if workspace == "" {
		return fmt.Errorf("workspace cannot be empty: %w", ErrInvalidToken)
	}

	if err := m.validateToken(token); err != nil {
		return err
	}

	// Create tokens directory if needed
	if err := fileutil.SecureMkdirAll(m.tokensDir, 0700); err != nil {
		return fmt.Errorf("create tokens directory: %w", err)
	}

	// Prepare token file
	tf := tokenFile{
		Token:     token,
		Workspace: workspace,
	}

	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}

	tokenPath := m.tokenPath(workspace)

	// Atomic write via temp file + rename to avoid TOCTOU symlink races.
	// If an attacker creates a symlink between tokenPath() returning and
	// the write, os.Rename replaces the symlink itself rather than following it.
	tmpFile, err := os.CreateTemp(m.tokensDir, ".token-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp token file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp token file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp token file: %w", err)
	}

	// Set secure permissions (0600: owner read-write only)
	if err := fileutil.SecureChmod(tmpPath, 0600); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod temp token file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, tokenPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp token file: %w", err)
	}

	m.logger.Debug("stored notion token", "workspace", workspace)
	return nil
}

// LoadToken retrieves a stored Notion API token for a workspace.
func (m *Manager) LoadToken(ctx context.Context, workspace string) (string, error) {
	// Context check
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	if workspace == "" {
		return "", fmt.Errorf("workspace cannot be empty: %w", ErrTokenNotFound)
	}

	tokenPath := m.tokenPath(workspace)
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("workspace %q: %w", workspace, ErrTokenNotFound)
		}
		return "", fmt.Errorf("read token file: %w", err)
	}

	var tf tokenFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return "", fmt.Errorf("parse token file: %w", err)
	}

	// Validate token format hasn't been corrupted
	if err := m.validateToken(tf.Token); err != nil {
		return "", fmt.Errorf("stored token is malformed: %w", err)
	}

	m.logger.Debug("loaded notion token", "workspace", workspace)
	return tf.Token, nil
}

// RevokeToken deletes a stored Notion API token for a workspace.
// Returns nil if the token doesn't exist.
func (m *Manager) RevokeToken(ctx context.Context, workspace string) error {
	// Context check
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if workspace == "" {
		return fmt.Errorf("workspace cannot be empty: %w", ErrInvalidToken)
	}

	tokenPath := m.tokenPath(workspace)
	err := os.Remove(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Already gone or never existed
			m.logger.Debug("token already revoked", "workspace", workspace)
			return nil
		}
		return fmt.Errorf("revoke token: %w", err)
	}

	m.logger.Debug("revoked notion token", "workspace", workspace)
	return nil
}

// ListTokens returns a list of all workspace IDs with stored tokens.
func (m *Manager) ListTokens(ctx context.Context) ([]string, error) {
	// Context check
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Check if directory exists
	if _, err := os.Stat(m.tokensDir); err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("check tokens directory: %w", err)
	}

	entries, err := os.ReadDir(m.tokensDir)
	if err != nil {
		return nil, fmt.Errorf("read tokens directory: %w", err)
	}

	var workspaces []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Skip temp files and non-notion tokens
		if strings.HasPrefix(name, ".token-") && strings.HasSuffix(name, ".tmp") {
			continue
		}

		if !strings.HasPrefix(name, "notion_") || !strings.HasSuffix(name, ".json") {
			continue
		}

		// Extract workspace ID
		workspace := strings.TrimPrefix(name, "notion_")
		workspace = strings.TrimSuffix(workspace, ".json")

		if workspace != "" {
			workspaces = append(workspaces, workspace)
		}
	}

	m.logger.Debug("listed notion tokens", "count", len(workspaces))
	return workspaces, nil
}

// validateToken checks if a token has the correct Notion format and length.
func (m *Manager) validateToken(token string) error {
	if token == "" {
		return fmt.Errorf("token cannot be empty: %w", ErrInvalidToken)
	}

	if !strings.HasPrefix(token, notionTokenPrefix) {
		return fmt.Errorf("token must start with %q: %w", notionTokenPrefix, ErrInvalidToken)
	}

	if len(token) < minTokenLength {
		return fmt.Errorf("token too short (got %d, expected at least %d): %w",
			len(token), minTokenLength, ErrInvalidToken)
	}

	return nil
}

// tokenPath returns the file path for a workspace's token.
// Workspace ID is sanitized to prevent path traversal attacks.
func (m *Manager) tokenPath(workspace string) string {
	safe := sanitizeWorkspaceID(workspace)
	path := filepath.Join(m.tokensDir, "notion_"+safe+".json")
	return filepath.Clean(path)
}

// sanitizeWorkspaceID sanitizes a workspace ID for use in a filename.
// Removes or replaces characters that could cause path traversal or filesystem issues.
func sanitizeWorkspaceID(workspace string) string {
	// Replace path separators and other problematic characters
	safe := strings.ReplaceAll(workspace, "/", "_")
	safe = strings.ReplaceAll(safe, "\\", "_")
	safe = strings.ReplaceAll(safe, "..", "_")
	safe = strings.ReplaceAll(safe, "\x00", "_")

	// Replace other control characters or problematic characters
	safe = strings.Map(func(r rune) rune {
		// Allow alphanumerics, hyphens, underscores, dots
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		// Replace anything else with underscore
		return '_'
	}, safe)

	return safe
}

// TokenPath returns the token file path for a workspace (for external use).
func (m *Manager) TokenPath(workspace string) string {
	return m.tokenPath(workspace)
}

// HasToken checks if a token exists for the given workspace.
func (m *Manager) HasToken(workspace string) bool {
	_, err := m.LoadToken(context.Background(), workspace)
	return err == nil
}
