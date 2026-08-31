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

package v1

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fatima-go/juno/domain"
	"github.com/fatima-go/juno/web"
	"github.com/gorilla/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClipReturnsLegacyClipboardWithoutBinaryAccept(t *testing.T) {
	controller := &clipControllerStub{clipboard: "legacy clipboard"}
	req := httptest.NewRequest(http.MethodPost, "/clip/v1", nil)
	res := httptest.NewRecorder()

	clip(controller, res, req)

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, web.HeaderValueContentType, res.Header().Get(web.HeaderContentType))
	assert.JSONEq(t, `{"content":"legacy clipboard"}`, res.Body.String())
}

func TestClipStreamsBinaryFile(t *testing.T) {
	dataFolder := t.TempDir()
	filename := "mydata.tar.gz"
	content := []byte{0x00, 0x01, 0x7f, 0xff}
	path := filepath.Join(dataFolder, filename)
	require.NoError(t, os.WriteFile(path, content, 0600))

	controller := &clipControllerStub{
		open: func(requested string) (*os.File, os.FileInfo, error) {
			assert.Equal(t, filename, requested)
			file, err := os.Open(path)
			if err != nil {
				return nil, nil, err
			}
			info, err := file.Stat()
			return file, info, err
		},
	}
	req := newBinaryClipRequest(t, filename)
	res := httptest.NewRecorder()

	clip(controller, res, req)

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, web.HeaderValueBinaryContentType, res.Header().Get(web.HeaderContentType))
	assert.Equal(t, content, res.Body.Bytes())
	assert.Equal(t, fmt.Sprint(len(content)), res.Header().Get("Content-Length"))

	disposition, params, err := mime.ParseMediaType(res.Header().Get(web.HeaderContentDisposition))
	require.NoError(t, err)
	assert.Equal(t, "attachment", disposition)
	assert.Equal(t, filename, params["filename"])
}

func TestClipBinaryErrorResponses(t *testing.T) {
	tests := []struct {
		name       string
		openError  error
		statusCode int
	}{
		{name: "invalid filename", openError: domain.ErrInvalidClipBinaryFilename, statusCode: http.StatusBadRequest},
		{name: "not regular", openError: domain.ErrClipBinaryFileNotRegular, statusCode: http.StatusBadRequest},
		{name: "too large", openError: domain.ErrClipBinaryFileTooLarge, statusCode: http.StatusRequestEntityTooLarge},
		{name: "missing", openError: os.ErrNotExist, statusCode: http.StatusNotFound},
		{name: "internal", openError: os.ErrPermission, statusCode: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller := &clipControllerStub{
				open: func(string) (*os.File, os.FileInfo, error) {
					return nil, nil, test.openError
				},
			}
			res := httptest.NewRecorder()

			clip(controller, res, newBinaryClipRequest(t, "mydata.tar.gz"))

			assert.Equal(t, test.statusCode, res.Code)
			assert.Equal(t, web.HeaderValueContentType, res.Header().Get(web.HeaderContentType))
			assert.NotEmpty(t, res.Body.String())
		})
	}
}

func TestClipBinaryRejectsInvalidJSON(t *testing.T) {
	controller := &clipControllerStub{}
	req := httptest.NewRequest(http.MethodPost, "/clip/v1", strings.NewReader("{"))
	req.Header.Set("Accept", web.HeaderValueBinaryContentType)
	res := httptest.NewRecorder()

	clip(controller, res, req)

	assert.Equal(t, http.StatusBadRequest, res.Code)
}

func TestHandleClipBinaryUsesMonitorRole(t *testing.T) {
	controller := &clipControllerStub{
		open: func(string) (*os.File, os.FileInfo, error) {
			return nil, nil, os.ErrNotExist
		},
	}
	handler := &Version1Handler{controller: controller}
	req := newBinaryClipRequest(t, "mydata.tar.gz")
	req.Header.Set(domain.HEADER_FATIMA_AUTH_TOKEN, "test-token")
	res := httptest.NewRecorder()

	handler.HandleClip(res, req)

	assert.EqualValues(t, domain.ROLE_MONITOR, controller.role)
	assert.Equal(t, "test-token", controller.token)
	assert.Equal(t, http.StatusNotFound, res.Code)
}

func TestClipBinaryUnlimitedDeadlineOverridesServerWriteTimeout(t *testing.T) {
	handler := handlers.LoggingHandler(io.Discard, http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		setClipBinaryWriteDeadline(res, 0)
		time.Sleep(50 * time.Millisecond)
		_, _ = fmt.Fprint(res, "complete")
	}))
	server := httptest.NewUnstartedServer(handler)
	server.Config.WriteTimeout = 10 * time.Millisecond
	server.Start()
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "complete", string(body))
}

func newBinaryClipRequest(t *testing.T, filename string) *http.Request {
	t.Helper()
	body := fmt.Sprintf(`{"filename":%q}`, filename)
	req := httptest.NewRequest(http.MethodPost, "/clip/v1", strings.NewReader(body))
	req.Header.Set("Accept", "text/plain, "+web.HeaderValueBinaryContentType)
	return req
}

type clipControllerStub struct {
	web.JunoWebServiceController
	clipboard string
	open      func(string) (*os.File, os.FileInfo, error)
	timeout   time.Duration
	role      domain.Role
	token     string
}

func (controller *clipControllerStub) ValidateToken(token string, role domain.Role) error {
	controller.token = token
	controller.role = role
	return nil
}

func (controller *clipControllerStub) GetClipboard() string {
	return controller.clipboard
}

func (controller *clipControllerStub) OpenClipBinaryFile(name string) (*os.File, os.FileInfo, error) {
	if controller.open == nil {
		return nil, nil, domain.ErrInvalidClipBinaryFilename
	}
	return controller.open(name)
}

func (controller *clipControllerStub) GetClipBinaryWriteTimeout() time.Duration {
	return controller.timeout
}
