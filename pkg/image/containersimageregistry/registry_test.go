package containersimageregistry

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.podman.io/image/v5/types"
)

func writeAuthFile(t *testing.T, dir, filename string, auths map[string]interface{}) string { //nolint:unparam
	t.Helper()
	data, err := json.Marshal(map[string]interface{}{"auths": auths})
	require.NoError(t, err)
	path := filepath.Join(dir, filename)
	require.NoError(t, os.WriteFile(path, data, 0600))
	return path
}

func authEntry(user, pass string) map[string]string { //nolint:unparam
	return map[string]string{
		"auth": base64.StdEncoding.EncodeToString([]byte(user + ":" + pass)),
	}
}

// hermeticCtx returns a SystemContext that isolates credential lookups
// from the host's /etc/containers/registries.conf.
func hermeticCtx(t *testing.T) *types.SystemContext {
	t.Helper()
	emptyDir := t.TempDir()
	regConf := filepath.Join(emptyDir, "registries.conf")
	require.NoError(t, os.WriteFile(regConf, []byte{}, 0600))
	return &types.SystemContext{
		SystemRegistriesConfPath:    regConf,
		SystemRegistriesConfDirPath: emptyDir,
	}
}

func TestGetAuthFile(t *testing.T) {
	const ref = "registry.example.com/ns/image"

	t.Run("REGISTRY_AUTH_FILE with matching creds", func(t *testing.T) {
		dir := t.TempDir()
		authPath := writeAuthFile(t, dir, "auth.json", map[string]interface{}{
			"registry.example.com": authEntry("user", "pass"),
		})
		t.Setenv("REGISTRY_AUTH_FILE", authPath)
		t.Setenv("DOCKER_CONFIG", t.TempDir())

		got := getAuthFile(hermeticCtx(t), ref)
		require.Equal(t, authPath, got)
	})

	t.Run("REGISTRY_AUTH_FILE without matching creds returns empty for fallback", func(t *testing.T) {
		dir := t.TempDir()
		authPath := writeAuthFile(t, dir, "auth.json", map[string]interface{}{
			"other.registry.io": authEntry("user", "pass"),
		})
		t.Setenv("REGISTRY_AUTH_FILE", authPath)
		t.Setenv("DOCKER_CONFIG", t.TempDir())

		got := getAuthFile(hermeticCtx(t), ref)
		require.Empty(t, got)
	})

	t.Run("REGISTRY_AUTH_FILE empty file returns empty for fallback", func(t *testing.T) {
		dir := t.TempDir()
		authPath := writeAuthFile(t, dir, "auth.json", map[string]interface{}{})
		t.Setenv("REGISTRY_AUTH_FILE", authPath)
		t.Setenv("DOCKER_CONFIG", t.TempDir())

		got := getAuthFile(hermeticCtx(t), ref)
		require.Empty(t, got)
	})

	t.Run("no env vars and no auth file returns empty", func(t *testing.T) {
		t.Setenv("REGISTRY_AUTH_FILE", "")
		t.Setenv("DOCKER_CONFIG", t.TempDir())

		got := getAuthFile(hermeticCtx(t), ref)
		require.Empty(t, got)
	})

	t.Run("falls back to sourceCtx.AuthFilePath", func(t *testing.T) {
		dir := t.TempDir()
		authPath := writeAuthFile(t, dir, "auth.json", map[string]interface{}{
			"registry.example.com": authEntry("user", "pass"),
		})
		t.Setenv("REGISTRY_AUTH_FILE", "")
		t.Setenv("DOCKER_CONFIG", t.TempDir())

		sysCtx := hermeticCtx(t)
		sysCtx.AuthFilePath = authPath
		got := getAuthFile(sysCtx, ref)
		require.Equal(t, authPath, got)
	})

	t.Run("REGISTRY_AUTH_FILE nonexistent file falls back to sourceCtx", func(t *testing.T) {
		dir := t.TempDir()
		authPath := writeAuthFile(t, dir, "auth.json", map[string]interface{}{
			"registry.example.com": authEntry("user", "pass"),
		})
		t.Setenv("REGISTRY_AUTH_FILE", "/nonexistent/auth.json")
		t.Setenv("DOCKER_CONFIG", t.TempDir())

		sysCtx := hermeticCtx(t)
		sysCtx.AuthFilePath = authPath
		got := getAuthFile(sysCtx, ref)
		require.Equal(t, authPath, got)
	})
}
