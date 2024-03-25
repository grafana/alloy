package alloyseed

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	os.Remove(legacyPath())
	exitVal := m.Run()
	os.Exit(exitVal)
}

func reset() {
	os.Remove(legacyPath())
	savedSeed = nil
	once = sync.Once{}
}

func TestNoExistingFile(t *testing.T) {
	t.Cleanup(reset)
	dir := t.TempDir()
	l := log.NewNopLogger()
	f := filepath.Join(dir, filename)
	require.NoFileExists(t, f)
	Init(dir, l)
	require.FileExists(t, f)
	loaded, err := readSeedFile(f, l)
	require.NoError(t, err)
	seed := Get()
	require.Equal(t, seed.UID, loaded.UID)
}

func TestExistingFile(t *testing.T) {
	t.Cleanup(reset)
	dir := t.TempDir()
	l := log.NewNopLogger()
	f := filepath.Join(dir, filename)
	seed := generateNew()
	writeSeedFile(seed, f, l)
	Init(dir, l)
	require.NotNil(t, savedSeed)
	require.Equal(t, seed.UID, savedSeed.UID)
	require.Equal(t, seed.UID, Get().UID)
}

func TestNoInitCalled(t *testing.T) {
	t.Cleanup(reset)
	l := log.NewNopLogger()
	seed := Get()
	require.NotNil(t, seed)
	f := legacyPath()
	require.FileExists(t, f)
	loaded, err := readSeedFile(f, l)
	require.NoError(t, err)
	require.Equal(t, seed.UID, loaded.UID)
}

func TestLegacyExists(t *testing.T) {
	t.Cleanup(reset)
	dir := t.TempDir()
	l := log.NewNopLogger()
	f := filepath.Join(dir, filename)
	seed := generateNew()
	writeSeedFile(seed, legacyPath(), l)
	Init(dir, l)
	require.FileExists(t, f)
	require.NotNil(t, savedSeed)
	require.Equal(t, seed.UID, savedSeed.UID)
	require.Equal(t, seed.UID, Get().UID)
	loaded, err := readSeedFile(f, l)
	require.NoError(t, err)
	require.Equal(t, seed.UID, loaded.UID)
}
