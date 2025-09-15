/*
 * Copyright 2025 github.com/fatima-go
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
 * @author dave
 * @date 25. 9. 13. 오후 10:05
 */

package goaway

import (
	"fmt"
	"time"

	. "github.com/fatima-go/fatima-core/ipc"
	log "github.com/fatima-go/fatima-log"
)

const (
	// goaway 메시지를 받은 이후 goawayStartTimeoutDuration 안에 goaway start 가 시작되어야 한다
	goawayStartTimeoutDuration = time.Millisecond * 200
	// goaway start 이후 goawayTimeoutDuration 안에 goaway done 이 완료되어야 한다
	goawayTimeoutDuration = time.Second * 31
)

func NewGoawayManager() FatimaIPCSessionListener {
	return &defaultGoawayListener{}
}

func ExecuteGoawayByIPC(proc string) error {
	if !IsFatimaIPCAvailable(proc) {
		return fmt.Errorf("ipc not available")
	}

	client, err := NewFatimaIPCClientSession(proc)
	if err != nil {
		return fmt.Errorf("cannot make connection to %s : %s", proc, err.Error())
	}

	defer client.Disconnect()

	message := NewMessageGoaway()
	err = client.SendCommand(message)
	if err != nil {
		return fmt.Errorf("fail to send cron execute : %s", err.Error())
	}

	log.Info("[%s] goaway sent to %s. transactionId=%s", client, proc, message.GetTransactionId())

	// receive goaway start
	err = handshakeGoawayStart(client)
	if err != nil {
		return fmt.Errorf("[%s] fail to handshake goaway start : %s", client, err.Error())
	}

	err = handshakeGoawayDone(client)
	if err != nil {
		// goaway start 를 받은 상태이므로 클라이언트에서 goaway 는 진행되었다고 간주한다
		log.Warn("[%s] fail to handshake goaway done : %s", client, err.Error())
	}

	return nil
}

func handshakeGoawayStart(client FatimaIPCClientSession) error {
	c1 := make(chan Message, 1)
	defer close(c1)

	go func() {
		// receive a response from a client
		clientMessage, e1 := client.ReadCommand()
		if e1 != nil {
			log.Warn("[%s] fail to read command : %s", client, e1.Error())
			return
		}
		c1 <- clientMessage
	}()

	// determine transaction from response is valid or not
	select {
	case clientMessage := <-c1:
		if !clientMessage.Is(CommandGoawayStart) {
			return fmt.Errorf("[%s] unexpected message from client : %s", client, clientMessage)
		}
		log.Info("[%s] goaway start received", client)
	case <-time.After(goawayStartTimeoutDuration):
		return fmt.Errorf("[%s] timeout to receive goaway start", client)
	}
	return nil
}

func handshakeGoawayDone(client FatimaIPCClientSession) error {
	c1 := make(chan Message, 1)
	defer close(c1)

	go func() {
		// receive a response from a client
		clientMessage, e1 := client.ReadCommand()
		if e1 != nil {
			log.Warn("[%s] fail to read command : %s", client, e1.Error())
			return
		}
		c1 <- clientMessage
	}()

	// determine transaction from response is valid or not
	select {
	case clientMessage := <-c1:
		if !clientMessage.Is(CommandGoawayDone) {
			return fmt.Errorf("[%s] unexpected message from client : %s", client, clientMessage)
		}
		log.Info("[%s] goaway done received", client)
	case <-time.After(goawayTimeoutDuration):
		return fmt.Errorf("[%s] timeout to receive goaway done", client)
	}
	return nil
}
