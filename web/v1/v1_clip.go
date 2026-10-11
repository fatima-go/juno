/*
 * Copyright 2023 github.com/fatima-go
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
 *
 * @project fatima-core
 * @author jin
 * @date 23. 4. 14. 오후 5:20
 */

package v1

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/juno/domain"
	"github.com/fatima-go/juno/web"
)

type Clipboard struct {
	Content string `json:"content"`
}

func clip(controller web.JunoWebServiceController, res http.ResponseWriter, req *http.Request) {
	if acceptsClipBinary(req) {
		clipBinary(controller, res, req)
		return
	}

	clipboard := Clipboard{}
	clipboard.Content = controller.GetClipboard()
	b, _ := json.Marshal(clipboard)
	web.ResponseSuccess(res, req, string(b))
}

func acceptsClipBinary(req *http.Request) bool {
	for _, value := range strings.Split(req.Header.Get("Accept"), ",") {
		mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
		if err == nil && mediaType == web.HeaderValueBinaryContentType {
			return true
		}
	}
	return false
}

func clipBinary(controller web.JunoWebServiceController, res http.ResponseWriter, req *http.Request) {
	params, err := parsingRequest(req)
	if err != nil {
		web.ResponseError(res, req, http.StatusBadRequest, "invalid request body")
		return
	}

	filename := params["filename"]
	file, info, err := controller.OpenClipBinaryFile(filename)
	if err != nil {
		writeClipBinaryError(res, req, filename, err)
		return
	}
	defer file.Close()

	setClipBinaryWriteDeadline(res, controller.GetClipBinaryWriteTimeout())
	content := io.NewSectionReader(file, 0, info.Size())
	web.ResponseBinary(res, req, filename, info.ModTime(), content)
}

func writeClipBinaryError(res http.ResponseWriter, req *http.Request, filename string, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidClipBinaryFilename), errors.Is(err, domain.ErrClipBinaryFileNotRegular):
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrClipBinaryFileTooLarge):
		web.ResponseError(res, req, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, os.ErrNotExist):
		web.ResponseError(res, req, http.StatusNotFound, "file not found: "+filename)
	default:
		log.Warn("fail to open clip binary file %q: %s", filename, err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, "fail to open binary file")
	}
}

type clipUploadResult struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

func clipUpload(controller web.JunoWebServiceController, res http.ResponseWriter, req *http.Request) {
	filename, err := clipUploadFilename(req)
	if err != nil {
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}
	overwrite := strings.EqualFold(strings.TrimSpace(req.Header.Get(web.HeaderFatimaClipOverwrite)), "true")

	body := newClipUploadBody(res, req.Body, controller.GetClipBinaryUploadTimeout())
	written, err := controller.SaveClipBinaryFile(filename, body, req.ContentLength, overwrite)
	// the upload and fsync may outlast any deadline, so give the response its own
	setClipBinaryWriteDeadline(res, clipUploadResponseTimeout)
	if err != nil {
		writeClipUploadError(res, req, filename, err)
		return
	}

	log.Info("clip binary uploaded : %s (%d bytes)", filename, written)
	b, _ := json.Marshal(clipUploadResult{Filename: filename, Size: written})
	web.ResponseSuccess(res, req, string(b))
}

func clipUploadFilename(req *http.Request) (string, error) {
	_, params, err := mime.ParseMediaType(req.Header.Get(web.HeaderContentDisposition))
	if err != nil || params["filename"] == "" {
		return "", errors.New("invalid content-disposition filename")
	}
	return params["filename"], nil
}

func writeClipUploadError(res http.ResponseWriter, req *http.Request, filename string, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidClipBinaryFilename),
		errors.Is(err, domain.ErrClipBinaryFileNotRegular),
		errors.Is(err, domain.ErrClipBinaryUploadIncomplete):
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrClipBinaryFileExists):
		web.ResponseError(res, req, http.StatusConflict, "file already exists: "+filename)
	case errors.Is(err, domain.ErrClipBinaryFileTooLarge):
		web.ResponseError(res, req, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, domain.ErrClipBinaryUploadBusy):
		web.ResponseError(res, req, http.StatusServiceUnavailable, err.Error())
	default:
		log.Warn("fail to save clip binary file %q: %s", filename, err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, "fail to save binary file")
	}
}

// newClipUploadBody replaces the server read timeout, which would otherwise cut
// off large uploads, with an idle timeout renewed on every read and an optional
// overall limit.
func newClipUploadBody(res http.ResponseWriter, body io.Reader, timeout time.Duration) io.Reader {
	reader := &clipUploadIdleReader{
		reader:     body,
		controller: http.NewResponseController(res),
		idle:       clipUploadIdleTimeout,
	}
	if timeout > 0 {
		reader.limit = time.Now().Add(timeout)
	}
	if err := reader.renewDeadline(); err != nil {
		if !errors.Is(err, http.ErrNotSupported) {
			log.Warn("fail to set clip binary read deadline: %s", err.Error())
		}
		return body
	}
	return reader
}

type clipUploadIdleReader struct {
	reader     io.Reader
	controller *http.ResponseController
	idle       time.Duration
	limit      time.Time
}

func (r *clipUploadIdleReader) Read(p []byte) (int, error) {
	_ = r.renewDeadline()
	return r.reader.Read(p)
}

func (r *clipUploadIdleReader) renewDeadline() error {
	deadline := time.Now().Add(r.idle)
	if !r.limit.IsZero() && r.limit.Before(deadline) {
		deadline = r.limit
	}
	return r.controller.SetReadDeadline(deadline)
}

const (
	clipUploadIdleTimeout     = 60 * time.Second
	clipUploadResponseTimeout = 15 * time.Second
)

func setClipBinaryWriteDeadline(res http.ResponseWriter, timeout time.Duration) {
	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	if err := http.NewResponseController(res).SetWriteDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		log.Warn("fail to set clip binary write deadline: %s", err.Error())
	}
}
