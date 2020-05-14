package taskrunner

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnectNativeHook_Name(t *testing.T) {
	t.Parallel()
	name := new(connectNativeHook).Name()
	require.Equal(t, "connect_native", name)
}

func setupCertDirs(t *testing.T) (string, string) {
	fd, err := ioutil.TempFile("", "connect_native_testcert")
	require.NoError(t, err)
	_, err = fd.WriteString("ABCDEF")
	require.NoError(t, err)
	err = fd.Close()
	require.NoError(t, err)

	d, err := ioutil.TempDir("", "connect_native_testsecrets")
	require.NoError(t, err)
	return fd.Name(), d
}

func cleanupCertDirs(t *testing.T, original, secrets string) {
	err := os.Remove(original)
	require.NoError(t, err)
	err = os.RemoveAll(secrets)
	require.NoError(t, err)
}

func TestConnectNativeHook_copyCertificate(t *testing.T) {
	t.Parallel()

	f, d := setupCertDirs(t)
	defer cleanupCertDirs(t, f, d)

	t.Run("no source", func(t *testing.T) {
		err := new(connectNativeHook).copyCertificate("", d, "out.pem")
		require.NoError(t, err)
	})

	t.Run("normal", func(t *testing.T) {
		err := new(connectNativeHook).copyCertificate(f, d, "out.pem")
		require.NoError(t, err)
		b, err := ioutil.ReadFile(filepath.Join(d, "out.pem"))
		require.NoError(t, err)
		require.Equal(t, "ABCDEF", string(b))
	})
}

func TestConnectNativeHook_copyCertificates(t *testing.T) {
	t.Parallel()
}
