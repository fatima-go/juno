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

	. "github.com/fatima-go/fatima-core/ipc"
)

func NewGoawayManager() FatimaIPCSessionListener {
	return &defaultGoawayListener{}
}

func executeGoawayByIPC(proc string) error {
	if !IsFatimaIPCAvailable(proc) {
		return fmt.Errorf("ipc not available")
	}

	client, err := NewFatimaIPCClientSession(proc)
	if err != nil {
		return fmt.Errorf("cannot make connection to %s : %s", proc, err.Error())
	}

	defer client.Disconnect()

	err = client.SendCommand(NewMessageGoaway())
	if err != nil {
		return fmt.Errorf("fail to send cron execute : %s", err.Error())
	}

	// TODO : receive goaway start/done

	return nil
}
