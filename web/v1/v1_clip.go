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

func setClipBinaryWriteDeadline(res http.ResponseWriter, timeout time.Duration) {
	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	if err := http.NewResponseController(res).SetWriteDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		log.Warn("fail to set clip binary write deadline: %s", err.Error())
	}
}
