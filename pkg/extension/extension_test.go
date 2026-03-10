package extension

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeExtFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadAllEmbeddedDefaults(t *testing.T) {
	// No local or global dirs — should return embedded defaults.
	exts, tags := LoadAll("")

	// Check some embedded tags exist.
	for _, tag := range []string{"review", "discuss", "explain", "critique", "propose", "test", "doc", "impl", "refactor"} {
		if _, ok := exts[tag]; !ok {
			t.Errorf("expected embedded tag %q", tag)
		}
		if _, ok := tags[tag]; !ok {
			t.Errorf("expected %q in TagSet", tag)
		}
	}

	// see/also should be in TagSet but not in the extension map.
	for _, tag := range []string{"see", "also"} {
		if _, ok := tags[tag]; !ok {
			t.Errorf("expected %q in TagSet", tag)
		}
		if _, ok := exts[tag]; ok {
			t.Errorf("expected %q NOT in extension map", tag)
		}
	}
}

func TestLoadAllLocalOverridesEmbedded(t *testing.T) {
	localDir := t.TempDir()
	extDir := filepath.Join(localDir, "extensions")
	writeExtFile(t, extDir, "review.yaml", "tag: review\nbehavior: custom local behavior\n")

	exts, _ := LoadAll(localDir)

	if exts["review"].Behavior != "custom local behavior" {
		t.Errorf("expected local override, got %q", exts["review"].Behavior)
	}
}

func TestLoadAllGlobalOverridesEmbedded(t *testing.T) {
	// Use a temp dir as config dir.
	globalDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", globalDir)

	extDir := filepath.Join(globalDir, "aireview", "extensions")
	writeExtFile(t, extDir, "review.yaml", "tag: review\nbehavior: custom global behavior\n")

	// Note: os.UserConfigDir on macOS uses ~/Library/Application Support, not XDG.
	// We test LoadExtension directly for global path coverage instead.
	ext := loadFile(filepath.Join(extDir, "review.yaml"))
	if ext == nil {
		t.Fatal("expected extension loaded")
	}
	if ext.Behavior != "custom global behavior" {
		t.Errorf("expected global override, got %q", ext.Behavior)
	}
}

func TestLoadAllLocalOverridesGlobal(t *testing.T) {
	localDir := t.TempDir()
	globalDir := t.TempDir()

	globalExtDir := filepath.Join(globalDir, "extensions")
	localExtDir := filepath.Join(localDir, "extensions")

	writeExtFile(t, globalExtDir, "mytag.yaml", "tag: mytag\nbehavior: global behavior\n")
	writeExtFile(t, localExtDir, "mytag.yaml", "tag: mytag\nbehavior: local behavior\n")

	// Simulate by loading global first, then local.
	exts := make(map[string]Extension)
	loadDir(globalExtDir, exts)
	loadDir(localExtDir, exts)

	if exts["mytag"].Behavior != "local behavior" {
		t.Errorf("expected local override, got %q", exts["mytag"].Behavior)
	}
}

func TestLoadExtensionSeeAlso(t *testing.T) {
	for _, tag := range []string{"see", "also"} {
		ext := LoadExtension(tag, "")
		if ext == nil {
			t.Fatalf("expected synthetic extension for %q", tag)
		}
		if ext.Behavior != "(cross-reference)" {
			t.Errorf("expected (cross-reference), got %q", ext.Behavior)
		}
	}
}

func TestLoadExtensionFromLocal(t *testing.T) {
	localDir := t.TempDir()
	extDir := filepath.Join(localDir, "extensions")
	writeExtFile(t, extDir, "custom.yaml", "tag: custom\nbehavior: my custom behavior\n")

	ext := LoadExtension("custom", localDir)
	if ext == nil {
		t.Fatal("expected extension loaded")
	}
	if ext.Behavior != "my custom behavior" {
		t.Errorf("expected custom behavior, got %q", ext.Behavior)
	}
}

func TestLoadExtensionFromEmbedded(t *testing.T) {
	ext := LoadExtension("review", "")
	if ext == nil {
		t.Fatal("expected embedded review extension")
	}
	if ext.Behavior == "" {
		t.Error("expected non-empty behavior")
	}
}

func TestLoadExtensionNotFound(t *testing.T) {
	ext := LoadExtension("nonexistent_tag_xyz", "")
	if ext != nil {
		t.Errorf("expected nil for unknown tag, got %+v", ext)
	}
}

func TestYAMLValidation(t *testing.T) {
	dir := t.TempDir()
	extDir := filepath.Join(dir, "extensions")

	tests := []struct {
		name     string
		filename string
		content  string
		loaded   bool
		stderr   string
	}{
		{
			name:     "valid file",
			filename: "valid.yaml",
			content:  "tag: valid\nbehavior: works\n",
			loaded:   true,
		},
		{
			name:     "malformed YAML",
			filename: "bad.yaml",
			content:  "tag: bad\n  invalid: {\n",
			loaded:   false,
			stderr:   "malformed YAML",
		},
		{
			name:     "zero byte file",
			filename: "empty.yaml",
			content:  "",
			loaded:   false,
		},
		{
			name:     "missing tag field",
			filename: "notag.yaml",
			content:  "behavior: something\n",
			loaded:   false,
			stderr:   "missing or empty tag",
		},
		{
			name:     "empty tag field",
			filename: "emptytag.yaml",
			content:  "tag: \"\"\nbehavior: something\n",
			loaded:   false,
			stderr:   "missing or empty tag",
		},
		{
			name:     "tag/filename mismatch",
			filename: "mismatch.yaml",
			content:  "tag: wrong\nbehavior: something\n",
			loaded:   false,
			stderr:   "does not match filename",
		},
		{
			name:     "extra unknown fields ignored",
			filename: "extra.yaml",
			content:  "tag: extra\nbehavior: works\nunknown_field: ignored\n",
			loaded:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean and recreate ext dir for each test.
			os.RemoveAll(extDir)
			writeExtFile(t, extDir, tt.filename, tt.content)

			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			exts := make(map[string]Extension)
			loadDir(extDir, exts)

			_ = w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			stderrOut := buf.String()

			expectedTag := tt.filename[:len(tt.filename)-len(".yaml")]
			_, found := exts[expectedTag]

			if tt.loaded && !found {
				t.Errorf("expected tag %q to be loaded", expectedTag)
			}
			if !tt.loaded && found {
				t.Errorf("expected tag %q NOT to be loaded", expectedTag)
			}
			if tt.stderr != "" && !bytes.Contains([]byte(stderrOut), []byte(tt.stderr)) {
				t.Errorf("expected stderr containing %q, got %q", tt.stderr, stderrOut)
			}
		})
	}
}

func TestLoadAllContinuesAfterMalformedFiles(t *testing.T) {
	localDir := t.TempDir()
	extDir := filepath.Join(localDir, "extensions")

	// Write a malformed file AND a valid file.
	writeExtFile(t, extDir, "bad.yaml", "tag: bad\n  invalid: {\n")
	writeExtFile(t, extDir, "good.yaml", "tag: good\nbehavior: works\n")

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	exts, tags := LoadAll(localDir)

	_ = w.Close()
	os.Stderr = oldStderr

	// good should be loaded, bad should not.
	if _, ok := exts["good"]; !ok {
		t.Error("expected 'good' to be loaded despite malformed sibling")
	}
	if _, ok := tags["good"]; !ok {
		t.Error("expected 'good' in TagSet")
	}
}
