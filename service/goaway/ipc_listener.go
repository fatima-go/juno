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
 * @date 25. 9. 13. 오후 10:23
 */

package goaway

import (
	. "github.com/fatima-go/fatima-core/ipc"
	log "github.com/fatima-go/fatima-log"
)

type defaultGoawayListener struct {
}

func (t *defaultGoawayListener) StartSession(ctx SessionContext) {
	log.Info("start session : %s", ctx)
}

func (t *defaultGoawayListener) OnReceiveCommand(ctx SessionContext, message Message) {
	log.Info("[sim] OnReceiveCommand : %s", message)

	if !message.Is(CommandTransactionVerify) {
		return
	}

	transactionId := AsString(message.Data.GetValue(DataKeyTransaction))
	if len(transactionId) == 0 {
		log.Warn("[%s] received empty transaction id", ctx)
		return
	}

	err := ctx.SendCommand(NewMessageTransactionVerifyDone(transactionId, true))
	if err != nil {
		log.Warn("fail to send transaction verify done : %s", err.Error())
	}
	log.Debug("[%s] sent transaction verify true : %s", ctx, transactionId)
}

func (t *defaultGoawayListener) OnClose(ctx SessionContext) {
	log.Info("OnClose : %s", ctx)
}
