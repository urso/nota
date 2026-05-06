package blocks

import (
	"testing"
)

func TestFromBytesAndRanges_NoRanges(t *testing.T) {
	content := []byte("line1\nline2\nline3\n")
	f := FromBytesAndRanges(content, nil)

	if len(f.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(f.Blocks))
	}
	if f.Blocks[0].IsReview {
		t.Error("expected code block")
	}
	if f.Blocks[0].StartLine != 1 {
		t.Errorf("expected StartLine 1, got %d", f.Blocks[0].StartLine)
	}
	if f.Blocks[0].Lines != 3 {
		t.Errorf("expected 3 lines, got %d", f.Blocks[0].Lines)
	}
}

func TestFromBytesAndRanges_SingleWholeLineComment(t *testing.T) {
	content := []byte("line1\n// review: test\nline3\n")
	ranges := []Range{{Start: 6, End: 22}}

	f := FromBytesAndRanges(content, ranges)

	if len(f.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(f.Blocks))
	}

	if f.Blocks[0].IsReview {
		t.Error("block 0 should be code")
	}
	if f.Blocks[0].StartLine != 1 {
		t.Errorf("block 0: expected StartLine 1, got %d", f.Blocks[0].StartLine)
	}

	if !f.Blocks[1].IsReview {
		t.Error("block 1 should be review")
	}
	if f.Blocks[1].StartLine != 2 {
		t.Errorf("block 1: expected StartLine 2, got %d", f.Blocks[1].StartLine)
	}

	if f.Blocks[2].IsReview {
		t.Error("block 2 should be code")
	}
	if f.Blocks[2].StartLine != 3 {
		t.Errorf("block 2: expected StartLine 3, got %d", f.Blocks[2].StartLine)
	}
}

func TestReviewBlocks(t *testing.T) {
	content := []byte("line1\n// review: a\nline3\n// review: b\nline5\n")
	ranges := []Range{
		{Start: 6, End: 19},
		{Start: 25, End: 38},
	}

	f := FromBytesAndRanges(content, ranges)
	indices := f.ReviewBlocks()

	if len(indices) != 2 {
		t.Fatalf("expected 2 review blocks, got %d", len(indices))
	}
	if indices[0] != 1 || indices[1] != 3 {
		t.Errorf("expected indices [1, 3], got %v", indices)
	}
}

func TestAdjustedLine_WholeLineComment(t *testing.T) {
	content := []byte("line1\n// review: test\nline3\n")
	ranges := []Range{{Start: 6, End: 22}}

	f := FromBytesAndRanges(content, ranges)

	// line3 is at original line 3, adjusted to line 2 after review removal
	if f.AdjustedLine(2) != 2 {
		t.Errorf("expected adjusted line 2, got %d", f.AdjustedLine(2))
	}

	// Verify Bytes excludes review
	result := f.Bytes()
	expected := []byte("line1\nline3\n")
	if string(result) != string(expected) {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestAdjustedLine_InlineComment(t *testing.T) {
	content := []byte("code // review: test\n")
	ranges := []Range{{Start: 4, End: 20}}

	f := FromBytesAndRanges(content, ranges)

	// Trailing newline block is at index 2, should stay at line 1
	if f.AdjustedLine(2) != 1 {
		t.Errorf("expected adjusted line 1, got %d", f.AdjustedLine(2))
	}

	result := f.Bytes()
	expected := []byte("code\n")
	if string(result) != string(expected) {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestAdjustedLine_MultipleComments(t *testing.T) {
	content := []byte("line1\n// review: a\nline3\n// review: b\nline5\n")
	ranges := []Range{
		{Start: 6, End: 19},
		{Start: 25, End: 38},
	}

	f := FromBytesAndRanges(content, ranges)

	// line3 (block 2): original line 3 → adjusted line 2
	if f.AdjustedLine(2) != 2 {
		t.Errorf("line3: expected adjusted line 2, got %d", f.AdjustedLine(2))
	}

	// line5 (block 4): original line 5 → adjusted line 3 (2 review lines removed)
	if f.AdjustedLine(4) != 3 {
		t.Errorf("line5: expected adjusted line 3, got %d", f.AdjustedLine(4))
	}

	result := f.Bytes()
	expected := []byte("line1\nline3\nline5\n")
	if string(result) != string(expected) {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestContextBefore(t *testing.T) {
	content := []byte("line1\nline2\n// review: test\nline4\n")
	ranges := []Range{{Start: 12, End: 28}}

	f := FromBytesAndRanges(content, ranges)

	ctx := f.ContextBefore(1, 3)
	expected := "line1\nline2\n"
	if string(ctx) != expected {
		t.Errorf("expected %q, got %q", expected, ctx)
	}
}

func TestContextAfter(t *testing.T) {
	content := []byte("line1\n// review: test\nline3\nline4\n")
	ranges := []Range{{Start: 6, End: 22}}

	f := FromBytesAndRanges(content, ranges)

	ctx := f.ContextAfter(1, 2)
	expected := "line3\nline4\n"
	if string(ctx) != expected {
		t.Errorf("expected %q, got %q", expected, ctx)
	}
}

func TestBlockComment(t *testing.T) {
	content := []byte("line1\n/* review: block\n   comment */\nline4\n")
	ranges := []Range{{Start: 6, End: 37}}

	f := FromBytesAndRanges(content, ranges)

	if len(f.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(f.Blocks))
	}

	if f.Blocks[1].Lines != 2 {
		t.Errorf("expected review block with 2 lines, got %d", f.Blocks[1].Lines)
	}
	if f.Blocks[1].StartLine != 2 {
		t.Errorf("expected review at line 2, got %d", f.Blocks[1].StartLine)
	}

	// line4 (block 2): original line 4 → adjusted line 2
	if f.AdjustedLine(2) != 2 {
		t.Errorf("expected adjusted line 2, got %d", f.AdjustedLine(2))
	}
}

func TestWholeLineComment(t *testing.T) {
	content := []byte("x := 1\n// review: message\ny := 2\n")
	ranges := []Range{{Start: 7, End: 26}}

	f := FromBytesAndRanges(content, ranges)

	if f.Blocks[1].StartLine != 2 {
		t.Errorf("expected review at line 2, got %d", f.Blocks[1].StartLine)
	}

	// y := 2 (block 2): adjusted line 2
	if f.AdjustedLine(2) != 2 {
		t.Errorf("expected adjusted line 2, got %d", f.AdjustedLine(2))
	}

	result := f.Bytes()
	expected := []byte("x := 1\ny := 2\n")
	if string(result) != string(expected) {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestTrailingComment(t *testing.T) {
	content := []byte("x := 1 // review: check\ny := 2\n")
	ranges := []Range{{Start: 6, End: 23}}

	f := FromBytesAndRanges(content, ranges)

	if len(f.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(f.Blocks))
	}

	if f.Blocks[1].StartLine != 1 {
		t.Errorf("expected review at line 1, got %d", f.Blocks[1].StartLine)
	}

	// Trailing part (block 2) stays at line 1
	if f.AdjustedLine(2) != 1 {
		t.Errorf("expected adjusted line 1, got %d", f.AdjustedLine(2))
	}

	result := f.Bytes()
	expected := []byte("x := 1\ny := 2\n")
	if string(result) != string(expected) {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestIndentedComment(t *testing.T) {
	content := []byte("func foo() {\n    // review: indented\n    x := 1\n}\n")
	ranges := []Range{{Start: 13, End: 37}}

	f := FromBytesAndRanges(content, ranges)

	if f.Blocks[1].StartLine != 2 {
		t.Errorf("expected review at line 2, got %d", f.Blocks[1].StartLine)
	}

	// x := 1 (block 2): adjusted line 2
	if f.AdjustedLine(2) != 2 {
		t.Errorf("expected adjusted line 2, got %d", f.AdjustedLine(2))
	}
}

func TestConsecutiveComments(t *testing.T) {
	content := []byte("line1\n// review: a\n// review: b\nline4\n")
	ranges := []Range{
		{Start: 6, End: 19},
		{Start: 19, End: 32},
	}

	f := FromBytesAndRanges(content, ranges)

	// line4 (block 3): original line 4 → adjusted line 2 (2 review lines removed)
	if f.AdjustedLine(3) != 2 {
		t.Errorf("expected adjusted line 2, got %d", f.AdjustedLine(3))
	}

	result := f.Bytes()
	expected := []byte("line1\nline4\n")
	if string(result) != string(expected) {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestCommentAtFileStart(t *testing.T) {
	content := []byte("// review: first line\ncode\n")
	ranges := []Range{{Start: 0, End: 22}}

	f := FromBytesAndRanges(content, ranges)

	if f.Blocks[0].StartLine != 1 {
		t.Errorf("expected review at line 1, got %d", f.Blocks[0].StartLine)
	}

	// code (block 1): adjusted line 1
	if f.AdjustedLine(1) != 1 {
		t.Errorf("expected adjusted line 1, got %d", f.AdjustedLine(1))
	}

	result := f.Bytes()
	expected := []byte("code\n")
	if string(result) != string(expected) {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestCommentAtFileEnd(t *testing.T) {
	content := []byte("code\n// review: last line\n")
	ranges := []Range{{Start: 5, End: 26}}

	f := FromBytesAndRanges(content, ranges)

	result := f.Bytes()
	expected := []byte("code\n")
	if string(result) != string(expected) {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestCommentAtFileEndNoNewline(t *testing.T) {
	content := []byte("code\n// review: no newline")
	ranges := []Range{{Start: 5, End: 26}}

	f := FromBytesAndRanges(content, ranges)

	result := f.Bytes()
	expected := []byte("code\n")
	if string(result) != string(expected) {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestMergedLineComments(t *testing.T) {
	content := []byte("x := 1\n// review: first line\n// second line\n// third line\ny := 2\n")
	ranges := []Range{{Start: 7, End: 58}}

	f := FromBytesAndRanges(content, ranges)

	if len(f.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(f.Blocks))
	}

	if f.Blocks[1].Lines != 3 {
		t.Errorf("expected review block with 3 lines, got %d", f.Blocks[1].Lines)
	}
	if f.Blocks[1].StartLine != 2 {
		t.Errorf("expected review at line 2, got %d", f.Blocks[1].StartLine)
	}

	if f.Blocks[2].StartLine != 5 {
		t.Errorf("expected y := 2 at line 5, got %d", f.Blocks[2].StartLine)
	}

	// y := 2 (block 2): adjusted line 2
	if f.AdjustedLine(2) != 2 {
		t.Errorf("expected adjusted line 2, got %d", f.AdjustedLine(2))
	}

	result := f.Bytes()
	expected := []byte("x := 1\ny := 2\n")
	if string(result) != string(expected) {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestOriginalBytes(t *testing.T) {
	content := []byte("line1\n// review: test\nline3\n")
	ranges := []Range{{Start: 6, End: 22}}

	f := FromBytesAndRanges(content, ranges)

	// OriginalBytes returns everything
	if string(f.OriginalBytes()) != string(content) {
		t.Errorf("OriginalBytes mismatch: expected %q, got %q", content, f.OriginalBytes())
	}

	// Bytes excludes review
	expected := []byte("line1\nline3\n")
	if string(f.Bytes()) != string(expected) {
		t.Errorf("Bytes mismatch: expected %q, got %q", expected, f.Bytes())
	}
}
