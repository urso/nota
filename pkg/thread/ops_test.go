package thread

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateID(t *testing.T) {
	tests := []struct {
		id      string
		wantErr bool
	}{
		{"l:0123456789abcdef", false},
		{"l:abcdef0123456789", false},
		{"gh:12345", false},
		{"gh:1", false},
		{"invalid", true},
		{"l:short", true},
		{"l:toolongid1234567890", true},
		{"gh:", true},
		{"gh:abc", true},
		{"x:0123456789abcdef", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			err := ValidateID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateID(%q) error = %v, wantErr = %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestFilename(t *testing.T) {
	tests := []struct {
		name string
		th   *Thread
		want string
	}{
		{
			name: "without group",
			th:   &Thread{ID: "l:0123456789abcdef", Number: 1},
			want: "001-0123456789abcdef.xml",
		},
		{
			name: "with group",
			th:   &Thread{ID: "l:0123456789abcdef", Number: 42, Group: "pr-123"},
			want: "pr-123-042-0123456789abcdef.xml",
		},
		{
			name: "three digit number",
			th:   &Thread{ID: "l:0123456789abcdef", Number: 123},
			want: "123-0123456789abcdef.xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Filename(tt.th)
			if got != tt.want {
				t.Errorf("Filename() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestParseAnchor(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	if err := os.WriteFile(testFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("valid anchor", func(t *testing.T) {
		anchor, err := ParseAnchor(testFile + ":10")
		if err != nil {
			t.Fatalf("ParseAnchor failed: %v", err)
		}
		if anchor.File != testFile {
			t.Errorf("file = %s, want %s", anchor.File, testFile)
		}
		if anchor.Line != 10 {
			t.Errorf("line = %d, want 10", anchor.Line)
		}
	})

	t.Run("missing line number", func(t *testing.T) {
		_, err := ParseAnchor(testFile)
		if err == nil {
			t.Error("expected error for missing line number")
		}
	})

	t.Run("invalid line number", func(t *testing.T) {
		_, err := ParseAnchor(testFile + ":abc")
		if err == nil {
			t.Error("expected error for invalid line number")
		}
	})

	t.Run("zero line number", func(t *testing.T) {
		_, err := ParseAnchor(testFile + ":0")
		if err == nil {
			t.Error("expected error for zero line number")
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, err := ParseAnchor("/nonexistent/file.go:10")
		if err == nil {
			t.Error("expected error for non-existent file")
		}
	})
}

func TestCreate(t *testing.T) {
	t.Run("creates thread with message", func(t *testing.T) {
		dir := t.TempDir()
		th, err := Create(dir, CreateOpts{
			Message: "Test message",
			Goal:    "review",
			Author:  "testuser",
		})
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if th.Status != "open" {
			t.Errorf("status = %s, want open", th.Status)
		}
		if th.Goal != "review" {
			t.Errorf("goal = %s, want review", th.Goal)
		}
		if len(th.Comments) != 1 {
			t.Fatalf("comments = %d, want 1", len(th.Comments))
		}
		if th.Comments[0].Bodies[0].Content != "Test message" {
			t.Errorf("content = %s, want 'Test message'", th.Comments[0].Bodies[0].Content)
		}
	})

	t.Run("rejects empty message", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Create(dir, CreateOpts{Message: ""})
		if err == nil {
			t.Error("expected error for empty message")
		}
	})

	t.Run("rejects invalid goal", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Create(dir, CreateOpts{
			Message: "test",
			Goal:    "invalid-goal",
		})
		if err == nil {
			t.Error("expected error for invalid goal")
		}
	})
}

func TestAddComment(t *testing.T) {
	t.Run("adds comment to thread", func(t *testing.T) {
		dir := t.TempDir()
		th, err := Create(dir, CreateOpts{
			Message: "Initial",
			Author:  "alice",
		})
		if err != nil {
			t.Fatal(err)
		}

		comment, err := AddComment(dir, th.ID, AddCommentOpts{
			Message: "Reply",
			Author:  "bob",
		})
		if err != nil {
			t.Fatalf("AddComment failed: %v", err)
		}
		if comment.Author != "bob" {
			t.Errorf("author = %s, want bob", comment.Author)
		}

		info, _ := FindThread(dir, th.ID)
		if len(info.Thread.Comments) != 2 {
			t.Errorf("comments = %d, want 2", len(info.Thread.Comments))
		}
	})

	t.Run("sets visibility local", func(t *testing.T) {
		dir := t.TempDir()
		th, _ := Create(dir, CreateOpts{Message: "Initial", Author: "alice"})

		comment, err := AddComment(dir, th.ID, AddCommentOpts{
			Message: "Local comment",
			Author:  "bob",
			Local:   true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if comment.Visibility != "local" {
			t.Errorf("visibility = %s, want local", comment.Visibility)
		}
	})

	t.Run("sets reply-to", func(t *testing.T) {
		dir := t.TempDir()
		th, _ := Create(dir, CreateOpts{Message: "Initial", Author: "alice"})
		replyTo := th.Comments[0].ID

		comment, err := AddComment(dir, th.ID, AddCommentOpts{
			Message: "Reply",
			Author:  "bob",
			ReplyTo: replyTo,
		})
		if err != nil {
			t.Fatal(err)
		}
		if comment.ReplyTo == nil || comment.ReplyTo.Ref != replyTo {
			t.Errorf("reply-to = %v, want %s", comment.ReplyTo, replyTo)
		}
	})

	t.Run("sets anchor on comment", func(t *testing.T) {
		dir := t.TempDir()
		testFile := filepath.Join(dir, "test.go")
		if err := os.WriteFile(testFile, []byte("package main\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		th, _ := Create(dir, CreateOpts{Message: "Initial", Author: "alice"})

		comment, err := AddComment(dir, th.ID, AddCommentOpts{
			Message: "Comment with anchor",
			Author:  "bob",
			Anchor:  testFile + ":5",
		})
		if err != nil {
			t.Fatal(err)
		}
		if comment.Anchor == nil {
			t.Fatal("expected anchor on comment")
		}
		if comment.Anchor.File != testFile {
			t.Errorf("anchor file = %s, want %s", comment.Anchor.File, testFile)
		}
		if comment.Anchor.Line != 5 {
			t.Errorf("anchor line = %d, want 5", comment.Anchor.Line)
		}

		// Verify it was persisted
		info, _ := FindThread(dir, th.ID)
		lastComment := info.Thread.Comments[len(info.Thread.Comments)-1]
		if lastComment.Anchor == nil {
			t.Error("anchor not persisted")
		}
	})

	t.Run("rejects invalid anchor", func(t *testing.T) {
		dir := t.TempDir()
		th, _ := Create(dir, CreateOpts{Message: "Initial", Author: "alice"})

		_, err := AddComment(dir, th.ID, AddCommentOpts{
			Message: "Comment",
			Author:  "bob",
			Anchor:  "/nonexistent/file.go:10",
		})
		if err == nil {
			t.Error("expected error for invalid anchor")
		}
	})

	t.Run("rejects empty message", func(t *testing.T) {
		dir := t.TempDir()
		th, _ := Create(dir, CreateOpts{Message: "Initial", Author: "alice"})

		_, err := AddComment(dir, th.ID, AddCommentOpts{Message: ""})
		if err == nil {
			t.Error("expected error for empty message")
		}
	})

	t.Run("rejects invalid thread ID", func(t *testing.T) {
		dir := t.TempDir()
		_, err := AddComment(dir, "invalid-id", AddCommentOpts{Message: "test"})
		if err == nil {
			t.Error("expected error for invalid thread ID")
		}
	})
}

func TestUpdateStatus(t *testing.T) {
	dir := t.TempDir()
	th, _ := Create(dir, CreateOpts{Message: "Test", Author: "alice"})

	tests := []struct {
		status string
	}{
		{"resolved"},
		{"wontfix"},
		{"open"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if err := UpdateStatus(dir, th.ID, tt.status); err != nil {
				t.Fatalf("UpdateStatus failed: %v", err)
			}
			info, _ := FindThread(dir, th.ID)
			if info.Thread.Status != tt.status {
				t.Errorf("status = %s, want %s", info.Thread.Status, tt.status)
			}
		})
	}
}

func TestUpdateGoal(t *testing.T) {
	t.Run("updates goal", func(t *testing.T) {
		dir := t.TempDir()
		th, _ := Create(dir, CreateOpts{Message: "Test", Goal: "review", Author: "alice"})

		if err := UpdateGoal(dir, th.ID, "impl"); err != nil {
			t.Fatalf("UpdateGoal failed: %v", err)
		}
		info, _ := FindThread(dir, th.ID)
		if info.Thread.Goal != "impl" {
			t.Errorf("goal = %s, want impl", info.Thread.Goal)
		}
	})

	t.Run("rejects invalid goal", func(t *testing.T) {
		dir := t.TempDir()
		th, _ := Create(dir, CreateOpts{Message: "Test", Author: "alice"})

		if err := UpdateGoal(dir, th.ID, "invalid"); err == nil {
			t.Error("expected error for invalid goal")
		}
	})
}

func TestSpawn(t *testing.T) {
	t.Run("creates child with parent reference", func(t *testing.T) {
		dir := t.TempDir()
		parent, _ := Create(dir, CreateOpts{Message: "Parent", Author: "alice", Group: "test-group"})

		child, err := Spawn(dir, parent.ID, SpawnOpts{
			Message: "Child",
			Author:  "bob",
		})
		if err != nil {
			t.Fatalf("Spawn failed: %v", err)
		}
		if child.Parent == nil {
			t.Fatal("child should have parent reference")
		}
		if child.Parent.Ref != parent.ID {
			t.Errorf("parent ref = %s, want %s", child.Parent.Ref, parent.ID)
		}
	})

	t.Run("inherits group from parent", func(t *testing.T) {
		dir := t.TempDir()
		parent, _ := Create(dir, CreateOpts{Message: "Parent", Author: "alice", Group: "inherited"})

		child, err := Spawn(dir, parent.ID, SpawnOpts{Message: "Child", Author: "bob"})
		if err != nil {
			t.Fatal(err)
		}
		if child.Group != "inherited" {
			t.Errorf("group = %s, want inherited", child.Group)
		}
	})

	t.Run("explicit group overrides parent", func(t *testing.T) {
		dir := t.TempDir()
		parent, _ := Create(dir, CreateOpts{Message: "Parent", Author: "alice", Group: "parent-group"})

		child, err := Spawn(dir, parent.ID, SpawnOpts{
			Message: "Child",
			Author:  "bob",
			Group:   "child-group",
		})
		if err != nil {
			t.Fatal(err)
		}
		if child.Group != "child-group" {
			t.Errorf("group = %s, want child-group", child.Group)
		}
	})
}

func TestAddDependencies(t *testing.T) {
	t.Run("adds single dependency", func(t *testing.T) {
		dir := t.TempDir()
		th, _ := Create(dir, CreateOpts{Message: "Test", Author: "alice"})

		dependsOn, err := AddDependencies(dir, th.ID, []string{"l:0123456789abcdef"})
		if err != nil {
			t.Fatalf("AddDependencies failed: %v", err)
		}
		if dependsOn != "l:0123456789abcdef" {
			t.Errorf("depends-on = %s, want l:0123456789abcdef", dependsOn)
		}
	})

	t.Run("adds multiple dependencies", func(t *testing.T) {
		dir := t.TempDir()
		th, _ := Create(dir, CreateOpts{Message: "Test", Author: "alice"})

		blockers := []string{"l:0123456789abcde0", "l:0123456789abcde1"}
		dependsOn, err := AddDependencies(dir, th.ID, blockers)
		if err != nil {
			t.Fatal(err)
		}
		if dependsOn != "l:0123456789abcde0,l:0123456789abcde1" {
			t.Errorf("depends-on = %s, want both blockers", dependsOn)
		}
	})

	t.Run("does not duplicate", func(t *testing.T) {
		dir := t.TempDir()
		th, _ := Create(dir, CreateOpts{Message: "Test", Author: "alice"})

		AddDependencies(dir, th.ID, []string{"l:0123456789abcdef"})
		dependsOn, _ := AddDependencies(dir, th.ID, []string{"l:0123456789abcdef"})

		if strings.Count(dependsOn, "l:0123456789abcdef") != 1 {
			t.Errorf("should not duplicate: %s", dependsOn)
		}
	})
}

func TestRemoveDependencies(t *testing.T) {
	t.Run("removes single dependency", func(t *testing.T) {
		dir := t.TempDir()
		th, _ := Create(dir, CreateOpts{Message: "Test", Author: "alice"})
		AddDependencies(dir, th.ID, []string{"l:0123456789abcdef"})

		dependsOn, err := RemoveDependencies(dir, th.ID, []string{"l:0123456789abcdef"})
		if err != nil {
			t.Fatalf("RemoveDependencies failed: %v", err)
		}
		if dependsOn != "" {
			t.Errorf("depends-on = %s, want empty", dependsOn)
		}
	})

	t.Run("removes one of multiple", func(t *testing.T) {
		dir := t.TempDir()
		th, _ := Create(dir, CreateOpts{Message: "Test", Author: "alice"})
		AddDependencies(dir, th.ID, []string{"l:0123456789abcde0", "l:0123456789abcde1"})

		dependsOn, err := RemoveDependencies(dir, th.ID, []string{"l:0123456789abcde0"})
		if err != nil {
			t.Fatal(err)
		}
		if dependsOn != "l:0123456789abcde1" {
			t.Errorf("depends-on = %s, want l:0123456789abcde1", dependsOn)
		}
	})
}

func TestParseDependsOn(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"l:abc123def456789", []string{"l:abc123def456789"}},
		{"l:abc123def456789,l:def456abc123789", []string{"l:abc123def456789", "l:def456abc123789"}},
		{" l:abc123def456789 , l:def456abc123789 ", []string{"l:abc123def456789", "l:def456abc123789"}},
		{"l:abc123def456789,,l:def456abc123789", []string{"l:abc123def456789", "l:def456abc123789"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseDependsOn(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("ParseDependsOn(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseDependsOn(%q)[%d] = %s, want %s", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
