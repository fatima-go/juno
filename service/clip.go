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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/fatima-go/fatima-core/v2"
	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/juno/domain"
)

const (
	PropClipBinaryMaxBytes                   = "clip.binary.max.bytes"
	PropClipBinaryWriteTimeoutSeconds        = "clip.binary.write.timeout.seconds"
	PropClipBinaryUploadTimeoutSeconds       = "clip.binary.upload.timeout.seconds"
	DefaultClipBinaryMaxBytes          int64 = 1 << 30
)

type clipBinaryConfig struct {
	maxBytes      int64
	writeTimeout  time.Duration
	uploadTimeout time.Duration
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

	loaded.writeTimeout = loadClipTimeout(config, PropClipBinaryWriteTimeoutSeconds)
	loaded.uploadTimeout = loadClipTimeout(config, PropClipBinaryUploadTimeoutSeconds)

	log.Info("clip binary config : max_bytes=%d, write_timeout=%s, upload_timeout=%s", loaded.maxBytes, loaded.writeTimeout, loaded.uploadTimeout)
	return loaded
}

func loadClipTimeout(config fatima.Config, key string) time.Duration {
	value, ok := config.GetValue(key)
	if !ok {
		return 0
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	maxDurationSeconds := int64(time.Duration(1<<63-1) / time.Second)
	if err != nil || seconds < 0 || seconds > maxDurationSeconds {
		log.Warn("invalid %s=%q. using unlimited timeout", key, value)
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func (service *DomainService) OpenClipBinaryFile(name string) (*os.File, os.FileInfo, error) {
	dataFolder := service.fatimaRuntime.GetEnv().GetFolderGuide().GetDataFolder()
	return openClipBinaryFile(dataFolder, name, service.clipBinaryConfig.maxBytes)
}

func (service *DomainService) GetClipBinaryWriteTimeout() time.Duration {
	return service.clipBinaryConfig.writeTimeout
}

func (service *DomainService) SaveClipBinaryFile(name string, body io.Reader, contentLength int64, overwrite bool) (int64, error) {
	if service.clipUploadSlots != nil {
		select {
		case service.clipUploadSlots <- struct{}{}:
			defer func() { <-service.clipUploadSlots }()
		default:
			return 0, domain.ErrClipBinaryUploadBusy
		}
	}
	dataFolder := service.fatimaRuntime.GetEnv().GetFolderGuide().GetDataFolder()
	return saveClipBinaryFile(dataFolder, name, body, contentLength, service.clipBinaryConfig.maxBytes, overwrite)
}

func (service *DomainService) GetClipBinaryUploadTimeout() time.Duration {
	return service.clipBinaryConfig.uploadTimeout
}

// saveClipBinaryFile writes the upload to a temporary file inside the data
// folder and moves it into place only after the whole body has arrived, so an
// interrupted upload never replaces an existing file.
func saveClipBinaryFile(dataFolder string, name string, body io.Reader, contentLength int64, maxBytes int64, overwrite bool) (int64, error) {
	if !isValidClipBinaryFilename(name) {
		return 0, fmt.Errorf("%w: %q", domain.ErrInvalidClipBinaryFilename, name)
	}
	if contentLength > maxBytes {
		return 0, fmt.Errorf("%w: %q (%d > %d bytes)", domain.ErrClipBinaryFileTooLarge, name, contentLength, maxBytes)
	}

	root, err := os.OpenRoot(dataFolder)
	if err != nil {
		return 0, err
	}
	defer root.Close()

	existing, err := checkClipUploadTarget(root, name, overwrite)
	if err != nil {
		return 0, err
	}

	tempName, err := clipUploadTempName()
	if err != nil {
		return 0, err
	}
	tempFile, err := root.OpenFile(tempName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return 0, err
	}
	closed := false
	committed := false
	defer func() {
		if !closed {
			_ = tempFile.Close()
		}
		if !committed {
			_ = root.Remove(tempName)
		}
	}()

	reader := &clipUploadReader{reader: io.LimitReader(body, maxBytes+1)}
	written, err := io.Copy(tempFile, reader)
	if reader.err != nil {
		return written, fmt.Errorf("%w: %q (%w)", domain.ErrClipBinaryUploadIncomplete, name, reader.err)
	}
	if err != nil {
		return written, err
	}
	if written > maxBytes {
		return written, fmt.Errorf("%w: %q (> %d bytes)", domain.ErrClipBinaryFileTooLarge, name, maxBytes)
	}
	if contentLength >= 0 && written != contentLength {
		return written, fmt.Errorf("%w: %q (expected %d bytes, received %d)", domain.ErrClipBinaryUploadIncomplete, name, contentLength, written)
	}
	if existing != nil {
		// keep the mode of the file being replaced
		if err = tempFile.Chmod(existing.Mode().Perm()); err != nil {
			return written, err
		}
	}
	if err = tempFile.Sync(); err != nil {
		return written, err
	}
	if err = tempFile.Close(); err != nil {
		return written, err
	}
	closed = true

	if overwrite {
		// re-check right before replacing so a directory or symlink created
		// during the upload is never swapped out
		if _, err = checkClipUploadTarget(root, name, true); err != nil {
			return written, err
		}
		if err = root.Rename(tempName, name); err != nil {
			return written, err
		}
		committed = true
		return written, nil
	}

	// a hard link fails when the target exists, which keeps no-overwrite
	// uploads from clobbering a file created during the transfer
	err = root.Link(tempName, name)
	if errors.Is(err, fs.ErrExist) {
		return written, fmt.Errorf("%w: %q", domain.ErrClipBinaryFileExists, name)
	}
	if err == nil {
		// the temp name still exists after a link and is removed by the deferred cleanup
		return written, nil
	}

	// filesystems without hard links fall back to check-then-rename
	log.Warn("fail to link clip binary file %q, falling back to rename: %s", name, err.Error())
	if _, err = checkClipUploadTarget(root, name, false); err != nil {
		return written, err
	}
	if err = root.Rename(tempName, name); err != nil {
		return written, err
	}
	committed = true
	return written, nil
}

// checkClipUploadTarget returns the info of an existing regular target file, or
// nil when the target does not exist.
func checkClipUploadTarget(root *os.Root, name string, overwrite bool) (os.FileInfo, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %q", domain.ErrClipBinaryFileNotRegular, name)
	}
	if !overwrite {
		return nil, fmt.Errorf("%w: %q", domain.ErrClipBinaryFileExists, name)
	}
	return info, nil
}

// removeStaleClipUploadFiles deletes temp files left by uploads that were cut
// off by a juno restart.
func removeStaleClipUploadFiles(dataFolder string) {
	matches, err := filepath.Glob(filepath.Join(dataFolder, clipUploadTempPrefix+"*"))
	if err != nil {
		return
	}
	for _, path := range matches {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if err = os.Remove(path); err != nil {
			log.Warn("fail to remove stale clip upload file %s: %s", path, err.Error())
			continue
		}
		log.Info("removed stale clip upload file %s", path)
	}
}

func clipUploadTempName() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return clipUploadTempPrefix + hex.EncodeToString(b), nil
}

// clipUploadReader remembers read failures so a broken client connection can be
// told apart from a local write failure.
type clipUploadReader struct {
	reader io.Reader
	err    error
}

func (r *clipUploadReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if err != nil && err != io.EOF {
		r.err = err
	}
	return n, err
}

const (
	clipUploadTempPrefix     = ".clip-upload-"
	maxConcurrentClipUploads = 4
)

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
	// in-flight upload temp files are never addressable by clients
	if strings.HasPrefix(name, clipUploadTempPrefix) {
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
