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

package domain

type LogLevelInfo struct {
	Level string `json:"level"`
	Name  string `json:"name"`
}

type LogLevelSummary struct {
	LogLevel []LogLevelInfo `json:"loglevels"`
	Name     string         `json:"package_name"`
}

type LogLevels struct {
	Group   string          `json:"package_group"`
	Host    string          `json:"package_host"`
	Summary LogLevelSummary `json:"summary"`
}

/*
{
	"package_group": "basic",
	"package_host": "xfp-stg",
	"summary": {
		"message": "1 process reflected to DEBUG",
		"package_name": "default"
	}
}
*/
