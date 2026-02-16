package notion

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setupTestManager creates a test manager with a temporary directory.
func setupTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	tempDir := t.TempDir()
	tokensDir := filepath.Join(tempDir, "tokens")
	logger := slog.Default()
	return NewManager(tokensDir, logger), tokensDir
}

// TestNewManager verifies NewManager creates a manager with correct fields.
func TestNewManager(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.Default()

	mgr := NewManager(tempDir, logger)

	if mgr.tokensDir != tempDir {
		t.Errorf("tokensDir: got %q, want %q", mgr.tokensDir, tempDir)
	}
	if mgr.logger == nil {
		t.Error("logger should not be nil")
	}
}

// TestNewManagerWithNilLogger verifies NewManager provides a default logger.
func TestNewManagerWithNilLogger(t *testing.T) {
	tempDir := t.TempDir()

	mgr := NewManager(tempDir, nil)

	if mgr.logger == nil {
		t.Error("logger should not be nil")
	}
}

// TestStoreTokenValid tests storing a valid Notion token.
func TestStoreTokenValid(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()
	workspace := "test-workspace"
	token := "ntn_" + strings.Repeat("a", 50)

	err := mgr.StoreToken(ctx, workspace, token)
	if err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}

	// Verify token was stored
	loaded, err := mgr.LoadToken(ctx, workspace)
	if err != nil {
		t.Fatalf("LoadToken failed: %v", err)
	}
	if loaded != token {
		t.Errorf("token: got %q, want %q", loaded, token)
	}
}

// TestStoreTokenDirectoryCreation verifies tokensDir is created if missing.
func TestStoreTokenDirectoryCreation(t *testing.T) {
	tempDir := t.TempDir()
	tokensDir := filepath.Join(tempDir, "nonexistent", "tokens")
	mgr := NewManager(tokensDir, slog.Default())
	ctx := context.Background()
	workspace := "test-workspace"
	token := "ntn_" + strings.Repeat("a", 50)

	err := mgr.StoreToken(ctx, workspace, token)
	if err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(tokensDir); err != nil {
		t.Errorf("directory not created: %v", err)
	}
}

// TestStoreTokenFilePermissions verifies token file has 0600 permissions.
func TestStoreTokenFilePermissions(t *testing.T) {
	mgr, tokensDir := setupTestManager(t)
	ctx := context.Background()
	workspace := "test-workspace"
	token := "ntn_" + strings.Repeat("a", 50)

	err := mgr.StoreToken(ctx, workspace, token)
	if err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}

	// Check file permissions
	tokenPath := mgr.TokenPath(workspace)
	info, err := os.Stat(tokenPath)
	if err != nil {
		t.Fatalf("stat token file: %v", err)
	}

	// Extract permission bits (last 9 bits)
	perms := info.Mode().Perm()
	expectedPerms := os.FileMode(0600)
	if perms != expectedPerms {
		t.Errorf("file permissions: got %o, want %o", perms, expectedPerms)
	}
}

// TestStoreTokenInvalidTokenEmpty tests rejection of empty token.
func TestStoreTokenInvalidTokenEmpty(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()
	workspace := "test-workspace"

	err := mgr.StoreToken(ctx, workspace, "")
	if err == nil {
		t.Error("should reject empty token")
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("error: got %v, want ErrInvalidToken", err)
	}
}

// TestStoreTokenInvalidTokenPrefix tests rejection of token without ntn_ prefix.
func TestStoreTokenInvalidTokenPrefix(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()
	workspace := "test-workspace"
	token := "invalid_" + strings.Repeat("a", 50)

	err := mgr.StoreToken(ctx, workspace, token)
	if err == nil {
		t.Error("should reject token without ntn_ prefix")
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("error: got %v, want ErrInvalidToken", err)
	}
}

// TestStoreTokenInvalidTokenTooShort tests rejection of token that's too short.
func TestStoreTokenInvalidTokenTooShort(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()
	workspace := "test-workspace"
	token := "ntn_short" // Only 9 chars total

	err := mgr.StoreToken(ctx, workspace, token)
	if err == nil {
		t.Error("should reject token that's too short")
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("error: got %v, want ErrInvalidToken", err)
	}
}

// TestStoreTokenEmptyWorkspace tests rejection of empty workspace.
func TestStoreTokenEmptyWorkspace(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()
	token := "ntn_" + strings.Repeat("a", 50)

	err := mgr.StoreToken(ctx, "", token)
	if err == nil {
		t.Error("should reject empty workspace")
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("error: got %v, want ErrInvalidToken", err)
	}
}

// TestLoadTokenSuccess tests successfully loading a stored token.
func TestLoadTokenSuccess(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()
	workspace := "test-workspace"
	token := "ntn_" + strings.Repeat("a", 50)

	// Store token first
	if err := mgr.StoreToken(ctx, workspace, token); err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}

	// Load token
	loaded, err := mgr.LoadToken(ctx, workspace)
	if err != nil {
		t.Fatalf("LoadToken failed: %v", err)
	}
	if loaded != token {
		t.Errorf("token: got %q, want %q", loaded, token)
	}
}

// TestLoadTokenNotFound tests LoadToken with non-existent workspace.
func TestLoadTokenNotFound(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()

	_, err := mgr.LoadToken(ctx, "nonexistent")
	if err == nil {
		t.Error("should return error for non-existent token")
	}
	if !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("error: got %v, want ErrTokenNotFound", err)
	}
}

// TestLoadTokenEmptyWorkspace tests LoadToken with empty workspace.
func TestLoadTokenEmptyWorkspace(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()

	_, err := mgr.LoadToken(ctx, "")
	if err == nil {
		t.Error("should return error for empty workspace")
	}
	if !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("error: got %v, want ErrTokenNotFound", err)
	}
}

// TestLoadTokenMalformed tests LoadToken with corrupted token file.
func TestLoadTokenMalformed(t *testing.T) {
	mgr, tokensDir := setupTestManager(t)
	ctx := context.Background()
	workspace := "test-workspace"

	// Create a corrupted token file
	tokenPath := mgr.TokenPath(workspace)
	if err := os.WriteFile(tokenPath, []byte("invalid json"), 0600); err != nil {
		t.Fatalf("write corrupted token: %v", err)
	}

	_, err := mgr.LoadToken(ctx, workspace)
	if err == nil {
		t.Error("should return error for malformed token file")
	}
}

// TestLoadTokenStoredMalformed tests LoadToken with token that fails validation.
func TestLoadTokenStoredMalformed(t *testing.T) {
	mgr, tokensDir := setupTestManager(t)
	ctx := context.Background()
	workspace := "test-workspace"

	// Manually create a valid JSON file with invalid token
	tf := tokenFile{
		Token:     "invalid_token",
		Workspace: workspace,
	}
	data, _ := json.Marshal(tf)
	tokenPath := mgr.TokenPath(workspace)
	if err := os.WriteFile(tokenPath, data, 0600); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	_, err := mgr.LoadToken(ctx, workspace)
	if err == nil {
		t.Error("should return error for stored malformed token")
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("error: got %v, want ErrInvalidToken", err)
	}
}

// TestRevokeTokenSuccess tests successfully revoking a token.
func TestRevokeTokenSuccess(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()
	workspace := "test-workspace"
	token := "ntn_" + strings.Repeat("a", 50)

	// Store token first
	if err := mgr.StoreToken(ctx, workspace, token); err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}

	// Revoke token
	if err := mgr.RevokeToken(ctx, workspace); err != nil {
		t.Fatalf("RevokeToken failed: %v", err)
	}

	// Verify token is gone
	_, err := mgr.LoadToken(ctx, workspace)
	if err == nil {
		t.Error("token should not exist after revocation")
	}
}

// TestRevokeTokenNotFound tests RevokeToken for non-existent token.
func TestRevokeTokenNotFound(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()

	// Should not error if token doesn't exist
	err := mgr.RevokeToken(ctx, "nonexistent")
	if err != nil {
		t.Errorf("RevokeToken should return nil for non-existent token, got %v", err)
	}
}

// TestRevokeTokenEmptyWorkspace tests RevokeToken with empty workspace.
func TestRevokeTokenEmptyWorkspace(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()

	err := mgr.RevokeToken(ctx, "")
	if err == nil {
		t.Error("should return error for empty workspace")
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("error: got %v, want ErrInvalidToken", err)
	}
}

// TestListTokensEmpty tests ListTokens with no stored tokens.
func TestListTokensEmpty(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()

	workspaces, err := mgr.ListTokens(ctx)
	if err != nil {
		t.Fatalf("ListTokens failed: %v", err)
	}
	if len(workspaces) != 0 {
		t.Errorf("expected empty list, got %d workspaces", len(workspaces))
	}
}

// TestListTokensMultiple tests ListTokens with multiple tokens.
func TestListTokensMultiple(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()

	// Store multiple tokens
	workspaceIds := []string{"workspace-1", "workspace-2", "workspace-3"}
	for _, wsId := range workspaceIds {
		token := "ntn_" + strings.Repeat("a", 50)
		if err := mgr.StoreToken(ctx, wsId, token); err != nil {
			t.Fatalf("StoreToken failed for %s: %v", wsId, err)
		}
	}

	// List tokens
	workspaces, err := mgr.ListTokens(ctx)
	if err != nil {
		t.Fatalf("ListTokens failed: %v", err)
	}

	if len(workspaces) != 3 {
		t.Errorf("expected 3 workspaces, got %d", len(workspaces))
	}

	// Verify all workspace IDs are in the list
	for _, wsId := range workspaceIds {
		found := false
		for _, ws := range workspaces {
			if ws == wsId {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("workspace %q not found in list", wsId)
		}
	}
}

// TestListTokensNoDirectory tests ListTokens when directory doesn't exist.
func TestListTokensNoDirectory(t *testing.T) {
	tempDir := t.TempDir()
	tokensDir := filepath.Join(tempDir, "nonexistent")
	mgr := NewManager(tokensDir, slog.Default())
	ctx := context.Background()

	workspaces, err := mgr.ListTokens(ctx)
	if err != nil {
		t.Fatalf("ListTokens failed: %v", err)
	}
	if len(workspaces) != 0 {
		t.Errorf("expected empty list for non-existent directory, got %d", len(workspaces))
	}
}

// TestListTokensIgnoreTempFiles tests ListTokens ignores temp files.
func TestListTokensIgnoreTempFiles(t *testing.T) {
	mgr, tokensDir := setupTestManager(t)
	ctx := context.Background()

	// Create directory
	if err := os.MkdirAll(tokensDir, 0700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// Store a valid token
	workspace := "workspace-1"
	token := "ntn_" + strings.Repeat("a", 50)
	if err := mgr.StoreToken(ctx, workspace, token); err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}

	// Create a temp file (should be ignored)
	tempPath := filepath.Join(tokensDir, ".token-temp.tmp")
	if err := os.WriteFile(tempPath, []byte("temp"), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	// List tokens
	workspaces, err := mgr.ListTokens(ctx)
	if err != nil {
		t.Fatalf("ListTokens failed: %v", err)
	}

	if len(workspaces) != 1 {
		t.Errorf("expected 1 workspace (temp file should be ignored), got %d", len(workspaces))
	}
}

// TestListTokensIgnoreNonNotionFiles tests ListTokens ignores non-notion files.
func TestListTokensIgnoreNonNotionFiles(t *testing.T) {
	mgr, tokensDir := setupTestManager(t)
	ctx := context.Background()

	// Create directory
	if err := os.MkdirAll(tokensDir, 0700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// Store a valid token
	workspace := "workspace-1"
	token := "ntn_" + strings.Repeat("a", 50)
	if err := mgr.StoreToken(ctx, workspace, token); err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}

	// Create a non-notion file (should be ignored)
	otherPath := filepath.Join(tokensDir, "other_token.json")
	if err := os.WriteFile(otherPath, []byte("{}"), 0600); err != nil {
		t.Fatalf("write other file: %v", err)
	}

	// List tokens
	workspaces, err := mgr.ListTokens(ctx)
	if err != nil {
		t.Fatalf("ListTokens failed: %v", err)
	}

	if len(workspaces) != 1 {
		t.Errorf("expected 1 workspace (non-notion file should be ignored), got %d", len(workspaces))
	}
}

// TestHasTokenExists tests HasToken for existing token.
func TestHasTokenExists(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()
	workspace := "test-workspace"
	token := "ntn_" + strings.Repeat("a", 50)

	// Store token first
	if err := mgr.StoreToken(ctx, workspace, token); err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}

	if !mgr.HasToken(workspace) {
		t.Error("HasToken should return true for existing token")
	}
}

// TestHasTokenNotFound tests HasToken for non-existent token.
func TestHasTokenNotFound(t *testing.T) {
	mgr, _ := setupTestManager(t)

	if mgr.HasToken("nonexistent") {
		t.Error("HasToken should return false for non-existent token")
	}
}

// TestContextCancellationStoreToken tests StoreToken respects context cancellation.
func TestContextCancellationStoreToken(t *testing.T) {
	mgr, _ := setupTestManager(t)
	workspace := "test-workspace"
	token := "ntn_" + strings.Repeat("a", 50)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := mgr.StoreToken(ctx, workspace, token)
	if err == nil {
		t.Error("should return error when context is cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error: got %v, want context.Canceled", err)
	}
}

// TestContextCancellationLoadToken tests LoadToken respects context cancellation.
func TestContextCancellationLoadToken(t *testing.T) {
	mgr, _ := setupTestManager(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := mgr.LoadToken(ctx, "workspace")
	if err == nil {
		t.Error("should return error when context is cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error: got %v, want context.Canceled", err)
	}
}

// TestContextCancellationRevokeToken tests RevokeToken respects context cancellation.
func TestContextCancellationRevokeToken(t *testing.T) {
	mgr, _ := setupTestManager(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := mgr.RevokeToken(ctx, "workspace")
	if err == nil {
		t.Error("should return error when context is cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error: got %v, want context.Canceled", err)
	}
}

// TestContextCancellationListTokens tests ListTokens respects context cancellation.
func TestContextCancellationListTokens(t *testing.T) {
	mgr, _ := setupTestManager(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := mgr.ListTokens(ctx)
	if err == nil {
		t.Error("should return error when context is cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error: got %v, want context.Canceled", err)
	}
}

// TestContextTimeout tests StoreToken respects context timeout.
func TestContextTimeout(t *testing.T) {
	mgr, _ := setupTestManager(t)
	workspace := "test-workspace"
	token := "ntn_" + strings.Repeat("a", 50)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Give timeout a moment to trigger
	time.Sleep(10 * time.Millisecond)

	err := mgr.StoreToken(ctx, workspace, token)
	if err == nil {
		t.Error("should return error when context times out")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error: got %v, want context.DeadlineExceeded", err)
	}
}

// TestSanitizeWorkspaceID tests workspace ID sanitization.
func TestSanitizeWorkspaceID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal-workspace", "normal-workspace"},
		{"workspace/with/slashes", "workspace_with_slashes"},
		{"workspace\\with\\backslashes", "workspace_with_backslashes"},
		{"workspace..with..dots", "workspace__with__dots"},
		{"workspace\x00with\x00null", "workspace_with_null"},
		{"WORKSPACE", "WORKSPACE"},
		{"workspace-123", "workspace-123"},
		{"workspace.123", "workspace.123"},
		{"workspace@special#chars", "workspace_special_chars"},
	}

	for _, test := range tests {
		result := sanitizeWorkspaceID(test.input)
		if result != test.expected {
			t.Errorf("sanitizeWorkspaceID(%q): got %q, want %q", test.input, result, test.expected)
		}
	}
}

// TestTokenPathSanitization tests that token path is properly sanitized.
func TestTokenPathSanitization(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager(tempDir, slog.Default())

	// Workspace with path traversal attempt
	workspace := "../../../etc/passwd"
	tokenPath := mgr.TokenPath(workspace)

	// Verify path is within tempDir
	if !strings.Contains(tokenPath, tempDir) {
		t.Errorf("token path escapes tempDir: %s", tokenPath)
	}

	// Verify path doesn't contain .. or /
	if strings.Contains(tokenPath, "..") || strings.Contains(tokenPath, "/etc/passwd") {
		t.Errorf("token path contains dangerous sequences: %s", tokenPath)
	}
}

// TestMultipleOperations tests sequence of store, load, list, revoke operations.
func TestMultipleOperations(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()

	workspaces := map[string]string{
		"workspace1": "ntn_" + strings.Repeat("1", 50),
		"workspace2": "ntn_" + strings.Repeat("2", 50),
		"workspace3": "ntn_" + strings.Repeat("3", 50),
	}

	// Store all tokens
	for ws, token := range workspaces {
		if err := mgr.StoreToken(ctx, ws, token); err != nil {
			t.Fatalf("StoreToken failed for %s: %v", ws, err)
		}
	}

	// List and verify count
	list1, err := mgr.ListTokens(ctx)
	if err != nil {
		t.Fatalf("ListTokens failed: %v", err)
	}
	if len(list1) != 3 {
		t.Errorf("expected 3 workspaces, got %d", len(list1))
	}

	// Load and verify each token
	for ws, token := range workspaces {
		loaded, err := mgr.LoadToken(ctx, ws)
		if err != nil {
			t.Fatalf("LoadToken failed for %s: %v", ws, err)
		}
		if loaded != token {
			t.Errorf("token mismatch for %s", ws)
		}
	}

	// Revoke one token
	if err := mgr.RevokeToken(ctx, "workspace2"); err != nil {
		t.Fatalf("RevokeToken failed: %v", err)
	}

	// List and verify count
	list2, err := mgr.ListTokens(ctx)
	if err != nil {
		t.Fatalf("ListTokens failed: %v", err)
	}
	if len(list2) != 2 {
		t.Errorf("expected 2 workspaces after revoke, got %d", len(list2))
	}

	// Verify revoked token is gone
	_, err = mgr.LoadToken(ctx, "workspace2")
	if err == nil {
		t.Error("revoked token should not be loadable")
	}
}

// TestTokenValidationEdgeCases tests edge cases in token validation.
func TestTokenValidationEdgeCases(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()
	workspace := "test"

	tests := []struct {
		name      string
		token     string
		shouldErr bool
	}{
		{"valid token", "ntn_" + strings.Repeat("a", 50), false},
		{"minimum length", "ntn_" + strings.Repeat("a", 36), false},
		{"just below minimum", "ntn_" + strings.Repeat("a", 35), true},
		{"empty string", "", true},
		{"only prefix", "ntn_", true},
		{"wrong prefix lowercase", "ntn_" + strings.Repeat("a", 50), false},
		{"wrong prefix uppercase", "NTN_" + strings.Repeat("a", 50), true},
		{"similar prefix", "ntnn_" + strings.Repeat("a", 50), true},
	}

	for _, test := range tests {
		err := mgr.StoreToken(ctx, workspace+test.name, test.token)
		if (err != nil) != test.shouldErr {
			t.Errorf("StoreToken(%s): shouldErr=%v, got err=%v", test.name, test.shouldErr, err)
		}
	}
}

// TestBenchmark_StoreAndLoadToken provides benchmark data for common operations.
func BenchmarkStoreToken(b *testing.B) {
	mgr, _ := setupTestManager(&testing.T{}, )
	ctx := context.Background()
	token := "ntn_" + strings.Repeat("a", 50)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		workspace := "workspace-" + string(rune(i))
		mgr.StoreToken(ctx, workspace, token)
	}
}

// BenchmarkLoadToken measures token loading performance.
func BenchmarkLoadToken(b *testing.B) {
	mgr, _ := setupTestManager(&testing.T{})
	ctx := context.Background()
	workspace := "test-workspace"
	token := "ntn_" + strings.Repeat("a", 50)
	mgr.StoreToken(ctx, workspace, token)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.LoadToken(ctx, workspace)
	}
}

// BenchmarkListTokens measures directory listing performance.
func BenchmarkListTokens(b *testing.B) {
	mgr, _ := setupTestManager(&testing.T{})
	ctx := context.Background()

	// Store multiple tokens
	for j := 0; j < 10; j++ {
		workspace := "workspace-" + string(rune(j))
		token := "ntn_" + strings.Repeat("a", 50)
		mgr.StoreToken(ctx, workspace, token)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.ListTokens(ctx)
	}
}
