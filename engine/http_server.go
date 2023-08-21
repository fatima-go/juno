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

package engine

import (
	"fmt"
	"github.com/fatima-go/fatima-core"
	"github.com/fatima-go/fatima-log"
	. "github.com/fatima-go/juno/domain"
	"github.com/fatima-go/juno/service"
	"github.com/fatima-go/juno/web"
	"github.com/fatima-go/juno/web/v1"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"strings"
	"time"
)

func NewWebServer(fatimaRuntime fatima.FatimaRuntime) *JunoHttpServer {
	server := new(JunoHttpServer)
	server.fatimaRuntime = fatimaRuntime
	return server
}

type JunoHttpServer struct {
	fatimaRuntime fatima.FatimaRuntime
	webService    *web.WebService
	domainService *service.DomainService
	router        *mux.Router
	loggingRouter http.Handler
	listenAddress string
}

func createDomainService(fatimaRuntime fatima.FatimaRuntime) *service.DomainService {
	service := service.NewDomainService(fatimaRuntime)

	return service
}

func (server *JunoHttpServer) Initialize() bool {
	log.Info("Juno HttpServer Initialize()")

	v, ok := server.fatimaRuntime.GetConfig().GetValue(PropWebServerAddress)
	if !ok {
		v = ""
	}
	server.listenAddress = v

	v, ok = server.fatimaRuntime.GetConfig().GetValue(PropWebServerPort)
	if !ok {
		v = "9180"
	}
	server.listenAddress = fmt.Sprintf("%s:%s", server.listenAddress, v)
	log.Info("web guard listen : %s", server.listenAddress)

	domainService := createDomainService(server.fatimaRuntime)
	server.domainService = domainService
	server.domainService.ListenAddress = server.listenAddress

	server.webService = web.GetWebService()
	server.webService.Regist(v1.NewWebService(domainService))
	server.domainService.UrlSeed = server.webService.GetUrlSeed()

	server.router = mux.NewRouter().StrictSlash(true)
	server.webService.GenerateSubRouter(server.router)

	server.loggingRouter = handlers.LoggingHandler(server, server.router)

	return true
}

func (server *JunoHttpServer) Write(p []byte) (n int, err error) {
	server.access(string(p[:len(p)-1]))
	return len(p), nil
}

func (server *JunoHttpServer) access(access string) {
	if len(access) < 10 {
		return
	}

	remote := strings.Split(access, " ")[0]
	idx := strings.Index(access, "/"+server.webService.GetUrlSeed())
	if idx < 1 {
		return
	}
	uri := strings.Split(access[idx:], " ")[0]
	log.Info("%s -> %s", remote, uri)
}

func (server *JunoHttpServer) Bootup() {
	log.Info("Juno HttpServer Bootup()")
}

func (server *JunoHttpServer) Shutdown() {
	server.domainService.UnregistJuno()
	log.Info("Juno HttpServer Shutdown()")
}

func (server *JunoHttpServer) GetType() fatima.FatimaComponentType {
	return fatima.COMP_READER
}

func (server *JunoHttpServer) StartListening() {
	log.Info("called StartListening()")

	srv := &http.Server{
		Handler:      server.loggingRouter,
		Addr:         server.listenAddress,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Info("start web guard listening...")
	go func() {
		server.domainService.RegistJuno()
	}()

	err := srv.ListenAndServe()
	if err != nil {
		log.Error("fail to start web guard : %s", err.Error())
		server.fatimaRuntime.Stop()
	}
}

func (server *JunoHttpServer) getGatewayAddress(suffix string) string {
	v, ok := server.fatimaRuntime.GetConfig().GetValue(PropGatewayServerAddress)
	if ok {
		addr := v
		v, ok = server.fatimaRuntime.GetConfig().GetValue(PropGatewayServerPort)
		if !ok {
			v = ValueGatewayDefaultPort
		}
		return fmt.Sprintf("http://%s:%s/%s", addr, v, suffix)
	}

	uri := os.Getenv(fatima.ENV_FATIMA_JUPITER_URI)
	if len(uri) == 0 {
		idx := strings.Index(server.listenAddress, ":")
		return fmt.Sprintf("http://%s:%s/%s", server.listenAddress[:idx], ValueGatewayDefaultPort, suffix)
	}

	if strings.HasSuffix(uri, "/") {
		return fmt.Sprintf("%s%s", uri, suffix)
	}
	return fmt.Sprintf("%s/%s", uri, suffix)
}
