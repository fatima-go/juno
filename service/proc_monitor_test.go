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

package service

import (
	"fmt"
	"testing"
	"time"
)

var countryTz = map[string]string{
	"Hungary": "Europe/Budapest",
	"Egypt":   "Africa/Cairo",
}

func Test(t *testing.T) {
	utc := time.UTC
	utc2, _ := time.LoadLocation("UTC")
	fmt.Printf("local : %s\n", time.Local)
	if utc == utc2 {
		fmt.Printf("seoul is same to local\n")
	} else {
		fmt.Printf("seoul is NOT same to local\n")
	}

}
