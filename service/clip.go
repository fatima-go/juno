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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/fatima-go/fatima-core"
	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/juno/domain"
)

const (
	PropClipBinaryMaxBytes                  = "clip.binary.max.bytes"
	PropClipBinaryWriteTimeoutSeconds       = "clip.binary.write.timeout.seconds"
	DefaultClipBinaryMaxBytes         int64 = 1 << 30
)

type clipBinaryConfig struct {
	maxBytes     int64
	writeTimeout time.Duration
}

func loadClipBinaryConfig(config fatima.Config) clipBinaryConfig {
	loaded := clipBinaryConfig{maxBytes: DefaultClipBinaryMaxBytes}

	if value, ok := config.GetValue(PropClipBinaryMaxBytes); ok {
		maxBytes, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil || maxBytes <= 0 || maxBytes > DefaultClipBinaryMaxBytes {
			log.Warn("invalid %s=%q. using default %d", PropClipBinaryMaxBytes, value, DefaultClipBinaryMaxBytes)
		} else {
			loaded.maxBytes = maxBytes
		}
	}

	if value, ok := config.GetValue(PropClipBinaryWriteTimeoutSeconds); ok {
		seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		maxDurationSeconds := int64(time.Duration(1<<63-1) / time.Second)
		if err != nil || seconds < 0 || seconds > maxDurationSeconds {
			log.Warn("invalid %s=%q. using unlimited timeout", PropClipBinaryWriteTimeoutSeconds, value)
		} else {
			loaded.writeTimeout = time.Duration(seconds) * time.Second
		}
	}

	log.Info("clip binary config : max_bytes=%d, write_timeout=%s", loaded.maxBytes, loaded.writeTimeout)
	return loaded
}

func (service *DomainService) OpenClipBinaryFile(name string) (*os.File, os.FileInfo, error) {
	dataFolder := service.fatimaRuntime.GetEnv().GetFolderGuide().GetDataFolder()
	return openClipBinaryFile(dataFolder, name, service.clipBinaryConfig.maxBytes)
}

func (service *DomainService) GetClipBinaryWriteTimeout() time.Duration {
	return service.clipBinaryConfig.writeTimeout
}

func openClipBinaryFile(dataFolder string, name string, maxBytes int64) (*os.File, os.FileInfo, error) {
	if !isValidClipBinaryFilename(name) {
		return nil, nil, fmt.Errorf("%w: %q", domain.ErrInvalidClipBinaryFilename, name)
	}

	root, err := os.OpenRoot(dataFolder)
	if err != nil {
		return nil, nil, err
	}
	defer root.Close()

	info, err := root.Lstat(name)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("%w: %q", domain.ErrClipBinaryFileNotRegular, name)
	}

	file, err := root.Open(name)
	if err != nil {
		return nil, nil, err
	}

	info, err = file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, nil, fmt.Errorf("%w: %q", domain.ErrClipBinaryFileNotRegular, name)
	}
	if info.Size() > maxBytes {
		file.Close()
		return nil, nil, fmt.Errorf("%w: %q (%d > %d bytes)", domain.ErrClipBinaryFileTooLarge, name, info.Size(), maxBytes)
	}

	return file, info, nil
}

func isValidClipBinaryFilename(name string) bool {
	if name == "" || name == "." || name == ".." || filepath.IsAbs(name) {
		return false
	}
	if filepath.Clean(name) != name || filepath.Base(name) != name || strings.ContainsAny(name, "/\\") {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
