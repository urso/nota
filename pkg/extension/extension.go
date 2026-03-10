package extension

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// validTagName matches the same pattern as the parser's pattern A tag capture group.
var validTagName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// Extension represents a tag extension with its behavior description.
type Extension struct {
	Tag      string `yaml:"tag"`
	Behavior string `yaml:"behavior"`
}

// TagSet is the set of known tag names.
type TagSet map[string]struct{}

// LoadAll loads all extensions from embedded defaults, global config dir, and local dir.
// Local overrides global overrides embedded. Returns nil error; warnings go to stderr.
func LoadAll(localDir string) (map[string]Extension, TagSet) {
	exts := make(map[string]Extension)

	// 1. Load embedded defaults.
	entries, err := embeddedFS.ReadDir("extensions")
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			data, err := embeddedFS.ReadFile("extensions/" + e.Name())
			if err != nil {
				continue
			}
			loadYAMLData(data, e.Name(), exts)
		}
	}

	// 2. Load global overrides from UserConfigDir.
	if globalDir, err := os.UserConfigDir(); err == nil {
		globalExtDir := filepath.Join(globalDir, "nota", "extensions")
		loadDir(globalExtDir, exts)
	}

	// 3. Load local overrides.
	if localDir != "" {
		localExtDir := filepath.Join(localDir, "extensions")
		loadDir(localExtDir, exts)
	}

	// Build TagSet: all extension tags + see/also.
	tags := make(TagSet)
	for tag := range exts {
		tags[tag] = struct{}{}
	}
	tags["see"] = struct{}{}
	tags["also"] = struct{}{}

	return exts, tags
}

// LoadExtension loads a single extension by tag name using the lookup chain.
// For "see"/"also", returns a synthetic cross-reference extension.
// Returns nil if not found or if tag name is invalid.
func LoadExtension(tag string, localDir string) *Extension {
	if tag == "see" || tag == "also" {
		return &Extension{Tag: tag, Behavior: "(cross-reference)"}
	}

	// Validate tag name to prevent path traversal and reject invalid names.
	if !validTagName.MatchString(tag) {
		return nil
	}

	// Try local first.
	if localDir != "" {
		if ext := loadFile(filepath.Join(localDir, "extensions", tag+".yaml")); ext != nil {
			return ext
		}
	}

	// Try global.
	if globalDir, err := os.UserConfigDir(); err == nil {
		if ext := loadFile(filepath.Join(globalDir, "nota", "extensions", tag+".yaml")); ext != nil {
			return ext
		}
	}

	// Try embedded.
	data, err := embeddedFS.ReadFile("extensions/" + tag + ".yaml")
	if err != nil {
		return nil
	}
	var ext Extension
	if err := yaml.Unmarshal(data, &ext); err != nil || ext.Tag == "" || ext.Tag != tag {
		return nil
	}
	return &ext
}

// loadDir scans a directory for .yaml extension files and adds/overrides into exts.
func loadDir(dir string, exts map[string]Extension) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot read extension %s: %v\n", path, err)
			continue
		}
		loadYAMLData(data, e.Name(), exts)
	}
}

// loadYAMLData parses a YAML extension file and adds it to exts.
// Validates tag field, filename match, etc. Skips with warning on errors.
func loadYAMLData(data []byte, filename string, exts map[string]Extension) {
	// 0-byte file: skip silently.
	if len(data) == 0 {
		return
	}

	var ext Extension
	if err := yaml.Unmarshal(data, &ext); err != nil {
		fmt.Fprintf(os.Stderr, "warning: malformed YAML in %s: %v\n", filename, err)
		return
	}

	if ext.Tag == "" {
		fmt.Fprintf(os.Stderr, "warning: missing or empty tag in %s\n", filename)
		return
	}

	if !validTagName.MatchString(ext.Tag) {
		fmt.Fprintf(os.Stderr, "warning: invalid tag name %q in %s (must match [a-z][a-z0-9_-]*)\n", ext.Tag, filename)
		return
	}

	expectedTag := strings.TrimSuffix(filename, ".yaml")
	if ext.Tag != expectedTag {
		fmt.Fprintf(os.Stderr, "warning: tag %q does not match filename %s\n", ext.Tag, filename)
		return
	}

	if ext.Behavior == "" {
		fmt.Fprintf(os.Stderr, "warning: empty behavior in %s\n", filename)
	}

	exts[ext.Tag] = ext
}

// loadFile loads a single extension YAML file, returning nil if not found or invalid.
func loadFile(path string) *Extension {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if len(data) == 0 {
		return nil
	}

	var ext Extension
	if err := yaml.Unmarshal(data, &ext); err != nil {
		return nil
	}

	expectedTag := strings.TrimSuffix(filepath.Base(path), ".yaml")
	if ext.Tag == "" || ext.Tag != expectedTag {
		return nil
	}
	return &ext
}
