/*
 * Copyright 2026 github.com/fatima-go
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fatima-go/juno/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadClipBinaryConfig(t *testing.T) {
	assert.Equal(t, int64(1<<30), DefaultClipBinaryMaxBytes)

	loaded := loadClipBinaryConfig(clipConfigStub{})
	assert.Equal(t, DefaultClipBinaryMaxBytes, loaded.maxBytes)
	assert.Zero(t, loaded.writeTimeout)
	assert.Zero(t, loaded.uploadTimeout)

	loaded = loadClipBinaryConfig(clipConfigStub{
		PropClipBinaryMaxBytes:             "2048",
		PropClipBinaryWriteTimeoutSeconds:  "5",
		PropClipBinaryUploadTimeoutSeconds: "7",
	})
	assert.Equal(t, int64(2048), loaded.maxBytes)
	assert.Equal(t, 5*time.Second, loaded.writeTimeout)
	assert.Equal(t, 7*time.Second, loaded.uploadTimeout)

	loaded = loadClipBinaryConfig(clipConfigStub{
		PropClipBinaryMaxBytes:             "0",
		PropClipBinaryWriteTimeoutSeconds:  "-1",
		PropClipBinaryUploadTimeoutSeconds: "x",
	})
	assert.Equal(t, DefaultClipBinaryMaxBytes, loaded.maxBytes)
	assert.Zero(t, loaded.writeTimeout)
	assert.Zero(t, loaded.uploadTimeout)

	loaded = loadClipBinaryConfig(clipConfigStub{
		PropClipBinaryMaxBytes: "1073741825",
	})
	assert.Equal(t, DefaultClipBinaryMaxBytes, loaded.maxBytes)
}

func TestOpenClipBinaryFile(t *testing.T) {
	dataFolder := t.TempDir()
	content := []byte{0x00, 0x01, 0x7f, 0xff}
	require.NoError(t, os.WriteFile(filepath.Join(dataFolder, "mydata.tar.gz"), content, 0600))

	file, info, err := openClipBinaryFile(dataFolder, "mydata.tar.gz", int64(len(content)))
	require.NoError(t, err)
	defer file.Close()

	actual, err := io.ReadAll(file)
	require.NoError(t, err)
	assert.Equal(t, content, actual)
	assert.Equal(t, int64(len(content)), info.Size())
}

func TestOpenClipBinaryFileRejectsUnsafeTargets(t *testing.T) {
	dataFolder := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dataFolder, "large.bin"), []byte("1234"), 0600))
	require.NoError(t, os.Mkdir(filepath.Join(dataFolder, "folder"), 0700))
	require.NoError(t, os.Symlink("large.bin", filepath.Join(dataFolder, "link.bin")))

	tests := []struct {
		name     string
		filename string
		expected error
	}{
		{name: "empty", filename: "", expected: domain.ErrInvalidClipBinaryFilename},
		{name: "parent", filename: "../secret", expected: domain.ErrInvalidClipBinaryFilename},
		{name: "absolute", filename: filepath.Join(dataFolder, "large.bin"), expected: domain.ErrInvalidClipBinaryFilename},
		{name: "backslash", filename: `folder\\large.bin`, expected: domain.ErrInvalidClipBinaryFilename},
		{name: "control", filename: "bad\nname", expected: domain.ErrInvalidClipBinaryFilename},
		{name: "directory", filename: "folder", expected: domain.ErrClipBinaryFileNotRegular},
		{name: "symlink", filename: "link.bin", expected: domain.ErrClipBinaryFileNotRegular},
		{name: "too large", filename: "large.bin", expected: domain.ErrClipBinaryFileTooLarge},
		{name: "missing", filename: "missing.bin", expected: os.ErrNotExist},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file, _, err := openClipBinaryFile(dataFolder, test.filename, 3)
			if file != nil {
				file.Close()
			}
			assert.ErrorIs(t, err, test.expected)
		})
	}
}

func TestSaveClipBinaryFile(t *testing.T) {
	dataFolder := t.TempDir()
	content := []byte{0x00, 0x01, 0x7f, 0xff}

	written, err := saveClipBinaryFile(dataFolder, "mydata.bin", bytes.NewReader(content), int64(len(content)), 1024, false)
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), written)
	assertClipFileContent(t, dataFolder, "mydata.bin", content)
	assertNoClipUploadTempFiles(t, dataFolder)

	written, err = saveClipBinaryFile(dataFolder, "empty.bin", strings.NewReader(""), -1, 1024, false)
	require.NoError(t, err)
	assert.Zero(t, written)
	assertClipFileContent(t, dataFolder, "empty.bin", []byte{})
	assertNoClipUploadTempFiles(t, dataFolder)
}

func TestSaveClipBinaryFileOverwrite(t *testing.T) {
	dataFolder := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dataFolder, "mydata.bin"), []byte("old"), 0600))

	_, err := saveClipBinaryFile(dataFolder, "mydata.bin", strings.NewReader("new"), 3, 1024, false)
	assert.ErrorIs(t, err, domain.ErrClipBinaryFileExists)
	assertClipFileContent(t, dataFolder, "mydata.bin", []byte("old"))

	_, err = saveClipBinaryFile(dataFolder, "mydata.bin", strings.NewReader("new"), 3, 1024, true)
	require.NoError(t, err)
	assertClipFileContent(t, dataFolder, "mydata.bin", []byte("new"))
	assertNoClipUploadTempFiles(t, dataFolder)
}

func TestSaveClipBinaryFileKeepsExistingMode(t *testing.T) {
	dataFolder := t.TempDir()
	path := filepath.Join(dataFolder, "run.sh")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0700))
	require.NoError(t, os.Chmod(path, 0750))

	_, err := saveClipBinaryFile(dataFolder, "run.sh", strings.NewReader("new"), 3, 1024, true)
	require.NoError(t, err)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0750), info.Mode().Perm())
}

func TestSaveClipBinaryFileRejectsWhenBusy(t *testing.T) {
	service := &DomainService{clipUploadSlots: make(chan struct{}, 1)}
	service.clipUploadSlots <- struct{}{}

	_, err := service.SaveClipBinaryFile("mydata.bin", strings.NewReader("data"), 4, false)
	assert.ErrorIs(t, err, domain.ErrClipBinaryUploadBusy)
}

func TestRemoveStaleClipUploadFiles(t *testing.T) {
	dataFolder := t.TempDir()
	stale := filepath.Join(dataFolder, clipUploadTempPrefix+"0123456789abcdef")
	kept := filepath.Join(dataFolder, "mydata.bin")
	require.NoError(t, os.WriteFile(stale, []byte("partial"), 0600))
	require.NoError(t, os.WriteFile(kept, []byte("data"), 0600))

	removeStaleClipUploadFiles(dataFolder)

	assertNoClipUploadTempFiles(t, dataFolder)
	assertClipFileContent(t, dataFolder, "mydata.bin", []byte("data"))
}

func TestSaveClipBinaryFilePreservesExistingFileOnFailure(t *testing.T) {
	dataFolder := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dataFolder, "mydata.bin"), []byte("old"), 0600))

	tests := []struct {
		name          string
		body          io.Reader
		contentLength int64
		expected      error
	}{
		{name: "declared too large", body: strings.NewReader("12345"), contentLength: 5, expected: domain.ErrClipBinaryFileTooLarge},
		{name: "streamed too large", body: strings.NewReader("12345"), contentLength: -1, expected: domain.ErrClipBinaryFileTooLarge},
		{name: "short body", body: strings.NewReader("12"), contentLength: 3, expected: domain.ErrClipBinaryUploadIncomplete},
		{name: "broken body", body: io.MultiReader(strings.NewReader("12"), errorReader{}), contentLength: 3, expected: domain.ErrClipBinaryUploadIncomplete},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := saveClipBinaryFile(dataFolder, "mydata.bin", test.body, test.contentLength, 4, true)
			assert.ErrorIs(t, err, test.expected)
			assertClipFileContent(t, dataFolder, "mydata.bin", []byte("old"))
			assertNoClipUploadTempFiles(t, dataFolder)
		})
	}
}

func TestSaveClipBinaryFileRejectsUnsafeTargets(t *testing.T) {
	dataFolder := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dataFolder, "folder"), 0700))
	require.NoError(t, os.Symlink(filepath.Join(outside, "target.bin"), filepath.Join(dataFolder, "link.bin")))

	tests := []struct {
		name     string
		filename string
		expected error
	}{
		{name: "empty", filename: "", expected: domain.ErrInvalidClipBinaryFilename},
		{name: "parent", filename: "../secret", expected: domain.ErrInvalidClipBinaryFilename},
		{name: "nested", filename: "folder/x.bin", expected: domain.ErrInvalidClipBinaryFilename},
		{name: "upload temp", filename: clipUploadTempPrefix + "0123456789abcdef", expected: domain.ErrInvalidClipBinaryFilename},
		{name: "directory", filename: "folder", expected: domain.ErrClipBinaryFileNotRegular},
		{name: "symlink", filename: "link.bin", expected: domain.ErrClipBinaryFileNotRegular},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := saveClipBinaryFile(dataFolder, test.filename, strings.NewReader("data"), 4, 1024, true)
			assert.ErrorIs(t, err, test.expected)
			assertNoClipUploadTempFiles(t, dataFolder)
		})
	}
	_, err := os.Stat(filepath.Join(outside, "target.bin"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func assertClipFileContent(t *testing.T, dataFolder string, name string, expected []byte) {
	t.Helper()
	actual, err := os.ReadFile(filepath.Join(dataFolder, name))
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func assertNoClipUploadTempFiles(t *testing.T, dataFolder string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dataFolder, clipUploadTempPrefix+"*"))
	require.NoError(t, err)
	assert.Empty(t, matches)
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("connection reset")
}

type clipConfigStub map[string]string

func (config clipConfigStub) GetValue(key string) (string, bool) {
	value, ok := config[key]
	return value, ok
}

func (config clipConfigStub) GetString(key string) (string, error) {
	if value, ok := config[key]; ok {
		return value, nil
	}
	return "", fmt.Errorf("not found: %s", key)
}

func (config clipConfigStub) GetInt(key string) (int, error) {
	return 0, fmt.Errorf("not implemented: %s", key)
}

func (config clipConfigStub) GetBool(key string) (bool, error) {
	return false, fmt.Errorf("not implemented: %s", key)
}

func (config clipConfigStub) GetList(key string) ([]string, error) {
	return nil, fmt.Errorf("not implemented: %s", key)
}
