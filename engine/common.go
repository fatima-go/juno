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
	"errors"
	"fmt"
	"os"
)

func ensureDirectory(path string, forceCreate bool) error {
	if stat, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			if forceCreate {
				return os.MkdirAll(path, 0755)
			}
		} else if !stat.IsDir() {
			return errors.New(fmt.Sprintf("%s path exist as file", path))
		}
	}

	return nil
}
