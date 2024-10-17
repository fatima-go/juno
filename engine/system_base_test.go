/*
 * Copyright 2024 github.com/fatima-go
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
 * @author dave_01
 * @date 24. 10. 15. 오후 4:06
 *
 */

package engine

import (
	"fmt"
	"sort"
	"testing"
)

func TestSort(t *testing.T) {
	weightList := make(ByWeight, 0)
	weightList = append(weightList, 0)
	weightList = append(weightList, 4)
	weightList = append(weightList, 0)
	weightList = append(weightList, 0)
	weightList = append(weightList, 1)
	weightList = append(weightList, 0)
	sort.Sort(weightList)
	fmt.Println(weightList)
}
