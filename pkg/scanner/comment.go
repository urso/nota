// Copyright 2023 Google LLC
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
// Adapted from github.com/ianlewis/todos.
// Original: internal/scanner/comment.go
// Changes: Package name updated. Added StartByte and EndByte fields for
// byte-offset tracking needed by the deleter.

package scanner

// Comment is a generic Comment implementation.
type Comment struct {
	Text      string
	Line      int
	Multiline bool

	// StartByte is the byte offset where the comment starts in the file.
	StartByte int64

	// EndByte is the byte offset where the comment ends in the file (exclusive).
	EndByte int64

	LineConfig      *LineCommentConfig
	MultilineConfig *MultilineCommentConfig
}

// String implements fmt.Stringer.String.
func (c *Comment) String() string {
	return c.Text
}
