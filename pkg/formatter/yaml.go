package formatter

import (
	"io"

	"github.com/urso/nota/pkg/grouper"
	"github.com/urso/nota/pkg/parser"
	"gopkg.in/yaml.v3"
)

type yamlGroup struct {
	Name       string          `yaml:"name"`
	Entries    []yamlEntry     `yaml:"entries,omitempty"`
	References []yamlReference `yaml:"references,omitempty"`
}

type yamlEntry struct {
	Tag     parser.Tag  `yaml:"tag"`
	File    string      `yaml:"file"`
	Line    int         `yaml:"line"`
	Comment string      `yaml:"comment"`
	Context yamlContext `yaml:"context"`
}

type yamlReference struct {
	File    string      `yaml:"file"`
	Line    int         `yaml:"line"`
	Context yamlContext `yaml:"context"`
}

type yamlContext struct {
	Before []string `yaml:"before"`
	After  []string `yaml:"after"`
}

// ensureContext returns a yamlContext with non-nil slices (emits [] instead of null).
func ensureContext(before, after []string) yamlContext {
	if before == nil {
		before = []string{}
	}
	if after == nil {
		after = []string{}
	}
	return yamlContext{Before: before, After: after}
}

// FormatYAML writes grouped review comments as YAML to w.
func FormatYAML(w io.Writer, groups []grouper.Group) error {
	out := make([]yamlGroup, 0, len(groups))

	for _, g := range groups {
		yg := yamlGroup{Name: g.Name}

		for _, e := range g.Entries {
			yg.Entries = append(yg.Entries, yamlEntry{
				Tag:     e.Tag,
				File:    e.File,
				Line:    e.Line,
				Comment: e.Comment,
				Context: ensureContext(e.Context.Before, e.Context.After),
			})
		}

		for _, r := range g.References {
			yg.References = append(yg.References, yamlReference{
				File:    r.File,
				Line:    r.Line,
				Context: ensureContext(r.Context.Before, r.Context.After),
			})
		}

		out = append(out, yg)
	}

	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	return enc.Encode(out)
}
