// Copyright 2023 Google LLC
// Copyright 2025 Ian Lewis
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
// Original: internal/scanner/scanner.go
// Changes: Package name updated. Imports updated to github.com/urso/aireview.
// Added byteOffset tracking and commentStartByte to populate StartByte/EndByte
// fields on Comment structs for byte-precise deletion support.

package scanner

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-enry/go-enry/v2"
	"github.com/ianlewis/runeio"
	"github.com/saintfish/chardet"
	"golang.org/x/text/encoding/ianaindex"
)

var (
	errDetectCharset = errors.New("detect charset")
	errDecodeCharset = errors.New("decoding charset")
)

var (
	// ErrUnsupportedLanguage indicates that the detected language is not supported.
	ErrUnsupportedLanguage = errors.New("unsupported language")

	// ErrBinaryFile indicates that the file is a binary file.
	ErrBinaryFile = errors.New("binary file")
)

// StringConfig is a configuration for a string literal.
type StringConfig struct {
	Start      []rune
	End        []rune
	EscapeFunc EscapeFunc
}

// LineCommentConfig is a configuration for a line comment.
type LineCommentConfig struct {
	Start       []rune
	AtLineStart bool
}

// MultilineCommentConfig is a configuration for a multi-line comment.
type MultilineCommentConfig struct {
	Start         []rune
	End           []rune
	AtFirstColumn bool
	Nested        bool
}

// Config is configuration for a generic comment scanner.
type Config struct {
	LineComments      []LineCommentConfig
	MultilineComments []MultilineCommentConfig
	Strings           []StringConfig
}

// FromFile returns an appropriate CommentScanner for the given file.
func FromFile(f *os.File, lang, charset string) (*ScanResult, error) {
	rawContents, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", f.Name(), err)
	}

	return FromBytes(f.Name(), rawContents, lang, charset)
}

// ScanResult holds both the scanner and the decoded file content.
// The decoded content should be used for all byte-offset operations (deletion, context extraction)
// since the scanner's byte offsets refer to positions in the decoded content.
type ScanResult struct {
	Scanner         *CommentScanner
	DecodedContents []byte
}

// FromBytes returns an appropriate CommentScanner and the decoded file contents.
func FromBytes(fileName string, rawContents []byte, lang, charset string) (*ScanResult, error) {
	if enry.IsBinary(rawContents) {
		return nil, ErrBinaryFile
	}

	if charset == "detect" {
		det := chardet.NewTextDetector()

		result, err := det.DetectBest(rawContents)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errDetectCharset, err)
		}

		charset = result.Charset
	}

	if charset == "ISO-8859-1" {
		charset = "UTF-8"
	}
	if charset == "GB-18030" {
		charset = "GB18030"
	}

	e, err := ianaindex.IANA.Encoding(charset)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", errDecodeCharset, charset, err)
	}

	if e == nil {
		return nil, fmt.Errorf("%w: %s: unsupported character set", errDecodeCharset, charset)
	}

	decodedContents, err := e.NewDecoder().Bytes(rawContents)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", errDecodeCharset, charset, err)
	}

	if lang != "" {
		lang, _ = enry.GetLanguageByAlias(lang)
	} else {
		lang = enry.GetLanguage(fileName, decodedContents)
	}

	if lang == enry.OtherLanguage {
		return nil, fmt.Errorf("%w: %s: language could not be detected", ErrUnsupportedLanguage, fileName)
	}

	config, ok := LanguagesConfig[lang]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLanguage, lang)
	}

	return &ScanResult{
		Scanner:         New(bytes.NewReader(decodedContents), config),
		DecodedContents: decodedContents,
	}, nil
}

// New returns a new CommentScanner that scans code returned by r with the given Config.
func New(r io.Reader, c *Config) *CommentScanner {
	return &CommentScanner{
		config: c,
		reader: runeio.NewReader(bufio.NewReader(r)),

		atLineStart:   true,
		atFirstColumn: true,

		state: &stateCode{},
		line:  1,
	}
}

// CommentScanner is a generic code comment scanner.
type CommentScanner struct {
	reader *runeio.RuneReader
	config *Config

	state state

	atFirstColumn bool
	atLineStart   bool

	line int

	// byteOffset tracks the current byte position in the input.
	byteOffset int64

	// commentStartByte records the byte offset at the start of a comment.
	commentStartByte int64

	next *Comment
	err  error
}

// Config returns the scanner's configuration.
func (s *CommentScanner) Config() *Config {
	return s.config
}

// Next returns the next Comment.
func (s *CommentScanner) Next() *Comment {
	return s.next
}

// Err returns an error if one occurred.
func (s *CommentScanner) Err() error {
	if errors.Is(s.err, io.EOF) {
		return nil
	}

	return s.err
}

// Scan implements a simple state machine to parse comments out of generic code.
func (s *CommentScanner) Scan() bool {
	for {
		if s.err != nil {
			return false
		}

		switch st := s.state.(type) {
		case *stateCode:
			s.state, s.err = s.processCode(st)
		case *stateString:
			s.state, s.err = s.processString(st)
		case *stateLineComment:
			s.state, s.err = s.processLineComment(st)
			if _, ok := s.state.(*stateLineComment); !ok {
				return true
			}
		case *stateLineCommentOrString:
			var hasComment bool

			hasComment, s.state, s.err = s.processLineCommentOrString(st)
			if hasComment {
				return true
			}
		case *stateMultilineComment:
			s.state, s.err = s.processMultilineComment(st)
			if _, ok := s.state.(*stateMultilineComment); !ok {
				return true
			}
		}
	}
}

//nolint:ireturn,nolintlint
func (s *CommentScanner) processCode(st *stateCode) (state, error) {
	for {
		lcIndex, m, err := s.lineMatch()
		if err != nil {
			return st, err
		}

		lcLen := 0
		if m != nil {
			lcLen = len(m.Start)
		}

		mmIndex, mm, err := s.multilineMatch()
		if err != nil {
			return st, err
		}

		mmLen := 0
		if mm != nil {
			mmLen = len(mm.Start)
		}

		sIndex, strs, err := s.stringMatch()
		if err != nil {
			return st, err
		}

		strsLen := 0
		if strs != nil {
			strsLen = len(strs.Start)
		}

		maxLen := max(lcLen, mmLen, strsLen)
		if maxLen == 0 {
			if _, err := s.nextRune(); err != nil {
				return st, err
			}

			continue
		}

		switch {
		case mmLen == maxLen && lcLen < maxLen && strsLen < maxLen:
			s.commentStartByte = s.byteOffset
			return &stateMultilineComment{
				line:  s.line,
				index: mmIndex,
			}, nil
		case lcLen == maxLen && mmLen < maxLen && strsLen < maxLen:
			s.commentStartByte = s.byteOffset
			return &stateLineComment{
				index: lcIndex,
			}, nil
		case strsLen == maxLen && lcLen < maxLen && mmLen < maxLen:
			return &stateString{
				index: sIndex,
			}, nil
		case mmLen == maxLen && lcLen == maxLen && strsLen < maxLen:
			s.commentStartByte = s.byteOffset
			return &stateMultilineComment{
				line:  s.line,
				index: mmIndex,
			}, nil
		case mmLen == maxLen && strsLen == maxLen && lcLen < maxLen:
			s.commentStartByte = s.byteOffset
			return &stateMultilineComment{
				line:  s.line,
				index: mmIndex,
			}, nil
		case lcLen == maxLen && strsLen == maxLen && mmLen < maxLen:
			s.commentStartByte = s.byteOffset
			return &stateLineCommentOrString{
				lcIndex: lcIndex,
				sIndex:  sIndex,
			}, nil
		default:
			s.commentStartByte = s.byteOffset
			return &stateMultilineComment{
				line:  s.line,
				index: mmIndex,
			}, nil
		}
	}
}

func (s *CommentScanner) lineMatch() (int, *LineCommentConfig, error) {
	for i, m := range s.config.LineComments {
		eq, err := s.peekEqual(m.Start)
		if err != nil {
			return 0, nil, err
		}

		if eq && (!m.AtLineStart || s.atLineStart) {
			return i, &m, nil
		}
	}

	return 0, nil, nil
}

func (s *CommentScanner) multilineMatch() (int, *MultilineCommentConfig, error) {
	for i, mlConfig := range s.config.MultilineComments {
		if eq, err := s.peekEqual(mlConfig.Start); err == nil && eq {
			if !mlConfig.AtFirstColumn || s.atFirstColumn {
				return i, &mlConfig, nil
			}
		} else if err != nil {
			return 0, nil, err
		}
	}

	return 0, nil, nil
}

func (s *CommentScanner) stringMatch() (int, *StringConfig, error) {
	for i, strs := range s.config.Strings {
		eq, err := s.peekEqual(strs.Start)
		if err != nil {
			return 0, nil, err
		}

		if eq {
			return i, &strs, nil
		}
	}

	return 0, nil, nil
}

//nolint:ireturn,nolintlint
func (s *CommentScanner) processString(st *stateString) (state, error) {
	if _, err := s.discardRunes(len(s.config.Strings[st.index].Start)); err != nil {
		return st, fmt.Errorf("parsing string: %w", err)
	}

	for {
		escaped, err := s.config.Strings[st.index].EscapeFunc(s, s.config.Strings[st.index].End)
		if err != nil && !errors.Is(err, io.EOF) {
			return st, err
		}

		if len(escaped) > 0 {
			if _, err := s.discardRunes(len(escaped)); err != nil {
				return st, fmt.Errorf("parsing string: %w", err)
			}
		} else {
			stringEnd, err := s.peekEqual(s.config.Strings[st.index].End)
			if err != nil {
				return st, fmt.Errorf("parsing string: %w", err)
			}

			if stringEnd {
				if _, err := s.discardRunes(len(s.config.Strings[st.index].End)); err != nil {
					return st, fmt.Errorf("parsing string: %w", err)
				}

				return &stateCode{}, nil
			}

			if _, err := s.nextRune(); err != nil {
				return st, fmt.Errorf("parsing string: %w", err)
			}
		}
	}
}

//nolint:ireturn,nolintlint
func (s *CommentScanner) processLineComment(st *stateLineComment) (state, error) {
	var b strings.Builder

	for {
		lineEnd, err := s.isLineEnd()
		if err != nil {
			return st, err
		}

		if lineEnd {
			s.next = &Comment{
				Text:       b.String(),
				Line:       s.line,
				Multiline:  false,
				StartByte:  s.commentStartByte,
				EndByte:    s.byteOffset,
				LineConfig: &s.config.LineComments[st.index],
			}

			return &stateCode{}, nil
		}

		rn, err := s.nextRune()
		if err != nil {
			return st, err
		}

		_, err = b.WriteRune(rn)
		if err != nil {
			return st, fmt.Errorf("writing rune %q: %w", rn, err)
		}
	}
}

//nolint:ireturn,nolintlint
func (s *CommentScanner) processLineCommentOrString(st *stateLineCommentOrString) (bool, state, error) {
	if _, err := s.discardRunes(len(s.config.Strings[st.sIndex].Start)); err != nil {
		return false, st, fmt.Errorf("parsing string: %w", err)
	}

	var commentTxt strings.Builder
	commentTxt.WriteString(string(s.config.Strings[st.sIndex].Start))

	for {
		lineEnd, err := s.isLineEnd()
		if err != nil {
			return false, st, err
		}

		if lineEnd {
			s.next = &Comment{
				Text:       commentTxt.String(),
				Line:       s.line,
				Multiline:  false,
				StartByte:  s.commentStartByte,
				EndByte:    s.byteOffset,
				LineConfig: &s.config.LineComments[st.lcIndex],
			}

			return true, &stateCode{}, nil
		}

		escaped, err := s.config.Strings[st.sIndex].EscapeFunc(
			s,
			s.config.Strings[st.sIndex].End,
		)
		if err != nil && !errors.Is(err, io.EOF) {
			return false, st, err
		}

		if len(escaped) > 0 {
			if _, discardErr := s.discardRunes(len(escaped)); discardErr != nil {
				return false, st, fmt.Errorf("parsing string: %w", discardErr)
			}

			_, err = commentTxt.WriteString(string(escaped))
			if err != nil {
				return false, st, fmt.Errorf("writing runes %q: %w", escaped, err)
			}

			continue
		}

		stringEnd, err := s.peekEqual(s.config.Strings[st.sIndex].End)
		if err != nil {
			return false, st, fmt.Errorf("parsing string: %w", err)
		}

		if stringEnd {
			if _, discardErr := s.discardRunes(len(s.config.Strings[st.sIndex].End)); discardErr != nil {
				return false, st, fmt.Errorf("parsing string: %w", discardErr)
			}

			return false, &stateCode{}, nil
		}

		rn, err := s.nextRune()
		if err != nil {
			return false, st, fmt.Errorf("parsing string: %w", err)
		}

		_, err = commentTxt.WriteRune(rn)
		if err != nil {
			return false, st, fmt.Errorf("writing rune %q: %w", rn, err)
		}
	}
}

//nolint:ireturn,nolintlint
func (s *CommentScanner) processMultilineComment(st *stateMultilineComment) (state, error) {
	mlConfig := s.config.MultilineComments[st.index]

	if _, errDiscard := s.discardRunes(len(mlConfig.Start)); errDiscard != nil {
		return st, fmt.Errorf("parsing code: %w", errDiscard)
	}

	var commentTxt strings.Builder

	var nestingDepth int

	commentTxt.WriteString(string(mlConfig.Start))

	for {
		if mlConfig.Nested {
			mlStart, err := s.peekEqual(mlConfig.Start)
			if err != nil {
				return st, err
			}

			if mlStart && (!mlConfig.AtFirstColumn || s.atFirstColumn) {
				if _, errDiscard := s.discardRunes(len(mlConfig.Start)); errDiscard != nil {
					return st, fmt.Errorf("parsing multi-line comment: %w", errDiscard)
				}
				commentTxt.WriteString(string(mlConfig.Start))

				nestingDepth++

				continue
			}
		}

		mlEnd, err := s.peekEqual(mlConfig.End)
		if err != nil {
			return st, err
		}

		if mlEnd && (!mlConfig.AtFirstColumn || s.atFirstColumn) {
			if _, errDiscard := s.discardRunes(len(mlConfig.End)); errDiscard != nil {
				return st, fmt.Errorf("parsing multi-line comment: %w", errDiscard)
			}
			commentTxt.WriteString(string(mlConfig.End))

			if nestingDepth == 0 {
				s.next = &Comment{
					Text:            commentTxt.String(),
					Line:            st.line,
					Multiline:       true,
					StartByte:       s.commentStartByte,
					EndByte:         s.byteOffset,
					MultilineConfig: &s.config.MultilineComments[st.index],
				}

				return &stateCode{}, nil
			}

			if mlConfig.Nested {
				nestingDepth--
			}
		}

		rn, err := s.nextRune()
		if err != nil {
			return st, err
		}

		_, err = commentTxt.WriteRune(rn)
		if err != nil {
			return st, fmt.Errorf("writing rune %q: %w", rn, err)
		}
	}
}

func (s *CommentScanner) nextRune() (rune, error) {
	rn, _, err := s.reader.ReadRune()
	if err != nil {
		return rn, fmt.Errorf("reading rune: %w", err)
	}

	s.byteOffset += int64(utf8.RuneLen(rn))

	if rn == '\n' {
		s.line++
		s.atFirstColumn = true
		s.atLineStart = true
	} else {
		s.atFirstColumn = false
		if !unicode.IsSpace(rn) {
			s.atLineStart = false
		}
	}

	return rn, nil
}

// discardRunes discards n runes from the reader, updating byteOffset.
func (s *CommentScanner) discardRunes(n int) (int, error) {
	peeked, err := s.reader.Peek(n)
	if err != nil {
		return 0, fmt.Errorf("peeking runes: %w", err)
	}

	for _, rn := range peeked {
		s.byteOffset += int64(utf8.RuneLen(rn))
		if rn == '\n' {
			s.line++
			s.atFirstColumn = true
			s.atLineStart = true
		} else {
			s.atFirstColumn = false
			if !unicode.IsSpace(rn) {
				s.atLineStart = false
			}
		}
	}

	return s.reader.Discard(n)
}

func (s *CommentScanner) isLineEnd() (bool, error) {
	nixNL, err := s.peekEqual([]rune{'\n'})
	if errors.Is(err, io.EOF) {
		return true, nil
	}

	if err != nil {
		return false, err
	}

	if nixNL {
		return true, nil
	}

	winNL, err := s.peekEqual([]rune{'\r', '\n'})
	if errors.Is(err, io.EOF) {
		return false, nil
	}

	return winNL, err
}

func (s *CommentScanner) peekEqual(val []rune) (bool, error) {
	r, err := s.reader.Peek(len(val))
	if err != nil {
		return false, fmt.Errorf("reading rune: %w", err)
	}

	return slices.Equal(r, val), nil
}
