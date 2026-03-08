// Copyright 2024 Google LLC
// Copyright 2025 Ian Lewis, Marcin Wiśniowski, Steffen Raabe
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
// Original: internal/scanner/languages.go
// Changes: Package name updated to scanner (from internal package).

package scanner

func concat[T any](slices ...[]T) []T {
	var tLen int
	for _, s := range slices {
		tLen += len(s)
	}

	newS := make([]T, tLen)

	var i int

	for _, s := range slices {
		i += copy(newS[i:], s)
	}

	return newS
}

var (
	hashLineComments = []LineCommentConfig{
		{Start: []rune("#")},
	}

	iniLineComments = []LineCommentConfig{
		{Start: []rune(";")},
	}

	cLineComments = []LineCommentConfig{
		{Start: []rune("//")},
	}

	cBlockComments = []MultilineCommentConfig{
		{Start: []rune("/*"), End: []rune("*/")},
	}

	singleQuoteString = []StringConfig{
		{Start: []rune{'\''}, End: []rune{'\''}, EscapeFunc: CharEscape('\\')},
	}

	doubleQuoteString = []StringConfig{
		{Start: []rune{'"'}, End: []rune{'"'}, EscapeFunc: CharEscape('\\')},
	}

	tripleDoubleQuoteString = []StringConfig{
		{Start: []rune(`"""`), End: []rune(`"""`), EscapeFunc: CharEscape('\\')},
	}

	cStrings = concat(doubleQuoteString, singleQuoteString)

	singleQuoteStringNoEscape = []StringConfig{
		{Start: []rune{'\''}, End: []rune{'\''}, EscapeFunc: NoEscape},
	}

	doubleQuoteStringNoEscape = []StringConfig{
		{Start: []rune{'"'}, End: []rune{'"'}, EscapeFunc: NoEscape},
	}

	fortranStrings = concat(doubleQuoteStringNoEscape, singleQuoteStringNoEscape)

	xmlBlockComments = []MultilineCommentConfig{
		{Start: []rune("<!--"), End: []rune("-->")},
	}

	tripleDoubleQuoteComments = []MultilineCommentConfig{
		{Start: []rune(`"""`), End: []rune(`"""`)},
	}
)

// LanguagesConfig is a map of language names to their configuration.
var LanguagesConfig = map[string]*Config{
	"Assembly": {
		LineComments:      iniLineComments,
		MultilineComments: cBlockComments,
		Strings: []StringConfig{
			{Start: []rune{'"'}, End: []rune{'"'}, EscapeFunc: NoEscape},
			{Start: []rune{'\''}, End: []rune{'\''}, EscapeFunc: NoEscape},
		},
	},
	"C": {
		LineComments:      cLineComments,
		MultilineComments: cBlockComments,
		Strings:           cStrings,
	},
	"C#": {
		LineComments:      cLineComments,
		MultilineComments: cBlockComments,
		Strings:           cStrings,
	},
	"C++": {
		LineComments:      cLineComments,
		MultilineComments: cBlockComments,
		Strings:           cStrings,
	},
	"Clojure": {
		LineComments: iniLineComments,
		Strings:      doubleQuoteString,
	},
	"CODEOWNERS": {
		LineComments: []LineCommentConfig{
			{Start: []rune("#"), AtLineStart: true},
		},
	},
	"CoffeeScript": {
		LineComments: hashLineComments,
		MultilineComments: []MultilineCommentConfig{
			{Start: []rune("###"), End: []rune("###")},
		},
		Strings: cStrings,
	},
	"Dart": {
		LineComments: concat(
			cLineComments,
			[]LineCommentConfig{{Start: []rune("///")}},
		),
		MultilineComments: cBlockComments,
		Strings:           cStrings,
	},
	"Dockerfile": {
		LineComments: hashLineComments,
		Strings:      cStrings,
	},
	"EditorConfig": {
		LineComments: hashLineComments,
	},
	"Elixir": {
		LineComments: hashLineComments,
		MultilineComments: []MultilineCommentConfig{
			{Start: []rune(`@moduledoc """`), End: []rune(`"""`)},
			{Start: []rune(`@doc """`), End: []rune(`"""`)},
		},
		Strings: concat(
			singleQuoteString,
			doubleQuoteString,
			tripleDoubleQuoteString,
			[]StringConfig{
				{Start: []rune("'''"), End: []rune("'''"), EscapeFunc: CharEscape('\\')},
			},
		),
	},
	"Emacs Lisp": {
		LineComments: iniLineComments,
		Strings:      doubleQuoteString,
	},
	"Erlang": {
		LineComments: []LineCommentConfig{
			{Start: []rune{'%'}},
		},
		Strings: cStrings,
	},
	"Fortran": {
		LineComments: []LineCommentConfig{
			{Start: []rune{'!'}},
		},
		Strings: fortranStrings,
	},
	"Fortran Free Form": {
		LineComments: []LineCommentConfig{
			{Start: []rune{'!'}},
		},
		Strings: fortranStrings,
	},
	"Git Config": {
		LineComments: concat(hashLineComments, iniLineComments),
		Strings:      doubleQuoteString,
	},
	"Go": {
		LineComments:      cLineComments,
		MultilineComments: cBlockComments,
		Strings: concat(
			cStrings,
			[]StringConfig{
				{Start: []rune{'`'}, End: []rune{'`'}, EscapeFunc: NoEscape},
			},
		),
	},
	"Go Module": {
		LineComments: cLineComments,
		Strings: concat(
			doubleQuoteString,
			[]StringConfig{
				{Start: []rune{'`'}, End: []rune{'`'}, EscapeFunc: NoEscape},
			},
		),
	},
	"GraphQL": {
		LineComments:      hashLineComments,
		MultilineComments: tripleDoubleQuoteComments,
		Strings:           doubleQuoteString,
	},
	"Groovy": {
		LineComments:      cLineComments,
		MultilineComments: cBlockComments,
		Strings: concat(
			cStrings,
			[]StringConfig{
				{Start: []rune("'''"), End: []rune("'''"), EscapeFunc: CharEscape('\\')},
			},
		),
	},
	"HCL": {
		LineComments: concat(hashLineComments, cLineComments),
		Strings:      doubleQuoteString,
	},
	"HTML": {
		MultilineComments: xmlBlockComments,
		Strings:           cStrings,
	},
	"HTML+ERB": {
		MultilineComments: concat(
			xmlBlockComments,
			[]MultilineCommentConfig{
				{Start: []rune("<%#"), End: []rune("%>")},
			},
		),
		Strings: cStrings,
	},
	"Haskell": {
		LineComments: []LineCommentConfig{
			{Start: []rune("--")},
		},
		MultilineComments: []MultilineCommentConfig{
			{Start: []rune("{-"), End: []rune("-}")},
		},
		Strings: cStrings,
	},
	"INI": {
		LineComments: concat(iniLineComments, hashLineComments),
		Strings:      cStrings,
	},
	"Ignore List": {
		LineComments: []LineCommentConfig{
			{Start: []rune("#"), AtLineStart: true},
		},
	},
	"JSON": {
		LineComments: []LineCommentConfig{
			{Start: []rune("//")},
			{Start: []rune{'#'}},
		},
		MultilineComments: cBlockComments,
		Strings:           cStrings,
	},
	"JSON5": {
		LineComments:      cLineComments,
		MultilineComments: cBlockComments,
		Strings:           cStrings,
	},
	"Java": {
		LineComments:      cLineComments,
		MultilineComments: cBlockComments,
		Strings:           cStrings,
	},
	"JavaScript": {
		LineComments:      cLineComments,
		MultilineComments: cBlockComments,
		Strings:           cStrings,
	},
	"Julia": {
		LineComments: hashLineComments,
		MultilineComments: []MultilineCommentConfig{
			{Start: []rune("#="), End: []rune("=#"), Nested: true},
		},
		Strings: concat(
			[]StringConfig{
				{Start: []rune(`raw"`), End: []rune(`"`), EscapeFunc: NoEscape},
			},
			cStrings,
			[]StringConfig{
				{Start: []rune(`raw"""`), End: []rune(`"""`), EscapeFunc: NoEscape},
			},
			tripleDoubleQuoteString,
		),
	},
	"Kotlin": {
		LineComments:      cLineComments,
		MultilineComments: cBlockComments,
		Strings:           cStrings,
	},
	"Lua": {
		LineComments: []LineCommentConfig{
			{Start: []rune("--")},
		},
		MultilineComments: []MultilineCommentConfig{
			{Start: []rune("--[["), End: []rune("--]]")},
		},
		Strings: cStrings,
	},
	"MATLAB": {
		LineComments: []LineCommentConfig{
			{Start: []rune{'%'}},
		},
		MultilineComments: []MultilineCommentConfig{
			{Start: []rune("%{"), End: []rune("}%")},
		},
		Strings: cStrings,
	},
	"Makefile": {
		LineComments: hashLineComments,
		Strings:      cStrings,
	},
	"Markdown": {
		MultilineComments: xmlBlockComments,
		Strings: []StringConfig{
			{Start: []rune{'`'}, End: []rune{'`'}, EscapeFunc: NoEscape},
			{Start: []rune("```"), End: []rune("```"), EscapeFunc: NoEscape},
		},
	},
	"Nix": {
		LineComments:      hashLineComments,
		MultilineComments: cBlockComments,
	},
	"Objective-C": {
		LineComments:      cLineComments,
		MultilineComments: cBlockComments,
		Strings:           cStrings,
	},
	"OCaml": {
		MultilineComments: []MultilineCommentConfig{
			{Start: []rune("(*"), End: []rune("*)"), Nested: true},
		},
		Strings: cStrings,
	},
	"Pascal": {
		LineComments: cLineComments,
		MultilineComments: []MultilineCommentConfig{
			{Start: []rune("(*"), End: []rune("*)")},
			{Start: []rune("{"), End: []rune("}")},
		},
		Strings: []StringConfig{
			{Start: []rune{'"'}, End: []rune{'"'}, EscapeFunc: DoubleEscape},
			{Start: []rune{'\''}, End: []rune{'\''}, EscapeFunc: DoubleEscape},
		},
	},
	"PHP": {
		LineComments: []LineCommentConfig{
			{Start: []rune{'#'}},
			{Start: []rune("//")},
		},
		MultilineComments: cBlockComments,
		Strings:           cStrings,
	},
	"Perl": {
		LineComments: hashLineComments,
		MultilineComments: []MultilineCommentConfig{
			{Start: []rune{'='}, End: []rune("=cut"), AtFirstColumn: true},
		},
		Strings: cStrings,
	},
	"Pip Requirements": {
		LineComments: hashLineComments,
	},
	"PowerShell": {
		LineComments: hashLineComments,
		MultilineComments: []MultilineCommentConfig{
			{Start: []rune("<#"), End: []rune("#>")},
		},
		Strings: []StringConfig{
			{Start: []rune{'"'}, End: []rune{'"'}, EscapeFunc: CharEscape('`')},
			{Start: []rune{'\''}, End: []rune{'\''}, EscapeFunc: CharEscape('`')},
		},
	},
	"Puppet": {
		LineComments: hashLineComments,
		Strings:      cStrings,
	},
	"Python": {
		LineComments:      hashLineComments,
		MultilineComments: tripleDoubleQuoteComments,
		Strings:           cStrings,
	},
	"R": {
		LineComments: hashLineComments,
		Strings:      cStrings,
	},
	"Ruby": {
		LineComments: hashLineComments,
		MultilineComments: []MultilineCommentConfig{
			{Start: []rune("=begin"), End: []rune("=end"), AtFirstColumn: true},
		},
		Strings: concat(
			cStrings,
			[]StringConfig{
				{Start: []rune("%{"), End: []rune{'}'}, EscapeFunc: CharEscape('\\')},
			},
		),
	},
	"Rust": {
		LineComments:      cLineComments,
		MultilineComments: cBlockComments,
		Strings:           doubleQuoteString,
	},
	"SQL": {
		LineComments: []LineCommentConfig{
			{Start: []rune("--")},
		},
		MultilineComments: cBlockComments,
		Strings: []StringConfig{
			{Start: []rune{'"'}, End: []rune{'"'}, EscapeFunc: DoubleEscape},
			{Start: []rune{'\''}, End: []rune{'\''}, EscapeFunc: DoubleEscape},
		},
	},
	"Scala": {
		LineComments:      cLineComments,
		MultilineComments: cBlockComments,
		Strings:           cStrings,
	},
	"Shell": {
		LineComments: hashLineComments,
		Strings:      cStrings,
	},
	"Svelte": {
		LineComments:      cLineComments,
		MultilineComments: concat(xmlBlockComments, cBlockComments),
		Strings:           cStrings,
	},
	"Swift": {
		LineComments:      cLineComments,
		MultilineComments: cBlockComments,
		Strings:           doubleQuoteString,
	},
	"TOML": {
		LineComments: hashLineComments,
		Strings:      cStrings,
	},
	"TeX": {
		LineComments: []LineCommentConfig{
			{Start: []rune{'%'}},
		},
	},
	"TypeScript": {
		LineComments:      cLineComments,
		MultilineComments: cBlockComments,
		Strings:           cStrings,
	},
	"Unix Assembly": {
		LineComments:      iniLineComments,
		MultilineComments: cBlockComments,
		Strings:           fortranStrings,
	},
	"VBA": {
		LineComments: []LineCommentConfig{
			{Start: []rune{'\''}},
		},
		Strings: doubleQuoteString,
	},
	"Vim Script": {
		LineComments: []LineCommentConfig{
			{Start: []rune{'"'}},
		},
		Strings: cStrings,
	},
	"Visual Basic .NET": {
		LineComments: []LineCommentConfig{
			{Start: []rune{'\''}},
		},
		Strings: doubleQuoteString,
	},
	"XML": {
		MultilineComments: xmlBlockComments,
		Strings:           cStrings,
	},
	"XSLT": {
		MultilineComments: xmlBlockComments,
		Strings:           cStrings,
	},
	"YAML": {
		LineComments: hashLineComments,
		Strings:      cStrings,
	},
}
