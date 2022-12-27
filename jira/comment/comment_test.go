// Copyright 2017 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package comment

import "testing"

//nolint:lll
const testComment = `Comment [(ID 484163403)|https://github.com] from GitHub user [bilbo-baggins|https://github.com/bilbo-baggins] (Bilbo Baggins) at 16:27 PM, April 17 2019:

Bla blibidy bloo bla`

func TestJiraCommentRegex(t *testing.T) {
	fields := jCommentRegex.FindStringSubmatch(testComment)

	if len(fields) != 6 {
		t.Fatalf("Regex failed to parse fields %v", fields)
	}

	if fields[1] != "484163403" {
		t.Fatalf("Expected field[1] = 484163403; Got field[1] = %s", fields[1])
	}

	if fields[2] != "bilbo-baggins" {
		t.Fatalf("Expected field[2] = bilbo-baggins; Got field[2] = %s", fields[2])
	}

	if fields[3] != "Bilbo Baggins" {
		t.Fatalf("Expected field[3] = Bilbo Baggins; Got field[3] = %s", fields[3])
	}

	if fields[4] != "16:27 PM, April 17 2019" {
		t.Fatalf("Expected field[4] = 16:27 PM, April 17 2019; Got field[4] = %s", fields[4])
	}

	if fields[5] != "Bla blibidy bloo bla" {
		t.Fatalf("Expected field[5] = Bla blibidy bloo bla; Got field[5] = %s", fields[5])
	}
}
