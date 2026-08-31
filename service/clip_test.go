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
	"fmt"
	"io"
	"os"
	"path/filepath"
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

	loaded = loadClipBinaryConfig(clipConfigStub{
		PropClipBinaryMaxBytes:            "2048",
		PropClipBinaryWriteTimeoutSeconds: "5",
	})
	assert.Equal(t, int64(2048), loaded.maxBytes)
	assert.Equal(t, 5*time.Second, loaded.writeTimeout)

	loaded = loadClipBinaryConfig(clipConfigStub{
		PropClipBinaryMaxBytes:            "0",
		PropClipBinaryWriteTimeoutSeconds: "-1",
	})
	assert.Equal(t, DefaultClipBinaryMaxBytes, loaded.maxBytes)
	assert.Zero(t, loaded.writeTimeout)

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
