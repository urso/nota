// Package blocks provides a block-based file representation for review comment extraction.
// Files are split into alternating code and review blocks, enabling O(n) line number
// tracking during iterative comment extraction.
package blocks

// Block represents a contiguous region of file content.
type Block struct {
	Content   []byte
	IsReview  bool
	StartLine int   // 1-indexed line number in original file
	StartByte int64 // byte offset in original file
	Lines     int   // number of newlines in Content
}

// File is a block-based representation of a source file.
type File struct {
	Blocks []Block
}

// FromBytesAndRanges creates a File from content and review comment byte ranges.
// Ranges must be non-overlapping and sorted by start position.
func FromBytesAndRanges(content []byte, ranges []Range) *File {
	if len(ranges) == 0 {
		return &File{
			Blocks: []Block{{
				Content:   content,
				IsReview:  false,
				StartLine: 1,
				StartByte: 0,
				Lines:     countLines(content),
			}},
		}
	}

	var blocks []Block
	line := 1
	var pos int64

	for _, r := range ranges {
		// Code block before this review comment.
		if r.Start > pos {
			code := content[pos:r.Start]
			blocks = append(blocks, Block{
				Content:   code,
				IsReview:  false,
				StartLine: line,
				StartByte: pos,
				Lines:     countLines(code),
			})
			line += countLines(code)
			pos = r.Start
		}

		// Review block.
		review := content[r.Start:r.End]
		blocks = append(blocks, Block{
			Content:   review,
			IsReview:  true,
			StartLine: line,
			StartByte: pos,
			Lines:     countLines(review),
		})
		line += countLines(review)
		pos = r.End
	}

	// Trailing code block.
	if pos < int64(len(content)) {
		code := content[pos:]
		blocks = append(blocks, Block{
			Content:   code,
			IsReview:  false,
			StartLine: line,
			StartByte: pos,
			Lines:     countLines(code),
		})
	}

	return &File{Blocks: blocks}
}

// Range is a byte range [Start, End) marking a review comment.
type Range struct {
	Start int64
	End   int64
}

// ReviewBlocks returns indices of all review blocks.
func (f *File) ReviewBlocks() []int {
	var indices []int
	for i, b := range f.Blocks {
		if b.IsReview {
			indices = append(indices, i)
		}
	}
	return indices
}

// AdjustedLine returns the line number for block i after all review blocks are removed.
func (f *File) AdjustedLine(i int) int {
	offset := 0
	for j := 0; j < i; j++ {
		if f.Blocks[j].IsReview {
			offset += f.lineDeltaForRemoval(j)
		}
	}
	return f.Blocks[i].StartLine - offset
}

// lineDeltaForRemoval computes how many lines are removed when block i is deleted.
func (f *File) lineDeltaForRemoval(i int) int {
	block := f.Blocks[i]

	startsAtLineStart := false
	if i == 0 {
		startsAtLineStart = true
	} else {
		prev := f.Blocks[i-1]
		if len(prev.Content) > 0 && prev.Content[len(prev.Content)-1] == '\n' {
			startsAtLineStart = true
		}
	}

	endsAtLineEnd := false
	if len(block.Content) > 0 && block.Content[len(block.Content)-1] == '\n' {
		endsAtLineEnd = true
	}

	if startsAtLineStart && endsAtLineEnd {
		return block.Lines
	}
	return 0
}

// Bytes returns file content with all review blocks removed.
func (f *File) Bytes() []byte {
	var size int
	for _, b := range f.Blocks {
		if !b.IsReview {
			size += len(b.Content)
		}
	}

	result := make([]byte, 0, size)
	for _, b := range f.Blocks {
		if !b.IsReview {
			result = append(result, b.Content...)
		}
	}
	return result
}

// OriginalBytes returns the original file content (all blocks).
func (f *File) OriginalBytes() []byte {
	var size int
	for _, b := range f.Blocks {
		size += len(b.Content)
	}

	result := make([]byte, 0, size)
	for _, b := range f.Blocks {
		result = append(result, b.Content...)
	}
	return result
}

// ContextBefore returns up to n lines of content before block i.
func (f *File) ContextBefore(i, n int) []byte {
	if i <= 0 || n <= 0 {
		return nil
	}

	var chunks [][]byte
	lines := 0

	for j := i - 1; j >= 0 && lines < n; j-- {
		b := f.Blocks[j]
		if b.IsReview {
			continue
		}
		// Extract up to n-lines lines from end of this block.
		extracted, count := extractLinesFromEnd(b.Content, n-lines)
		if len(extracted) > 0 {
			chunks = append([][]byte{extracted}, chunks...)
			lines += count
		}
	}

	return joinBytes(chunks)
}

// ContextAfter returns up to n lines of content after block i.
func (f *File) ContextAfter(i, n int) []byte {
	if i >= len(f.Blocks)-1 || n <= 0 {
		return nil
	}

	var chunks [][]byte
	lines := 0

	for j := i + 1; j < len(f.Blocks) && lines < n; j++ {
		b := f.Blocks[j]
		if b.IsReview {
			continue
		}
		// Extract up to n-lines lines from start of this block.
		extracted, count := extractLinesFromStart(b.Content, n-lines)
		if len(extracted) > 0 {
			chunks = append(chunks, extracted)
			lines += count
		}
	}

	return joinBytes(chunks)
}

func countLines(b []byte) int {
	count := 0
	for _, c := range b {
		if c == '\n' {
			count++
		}
	}
	return count
}

func extractLinesFromEnd(content []byte, n int) ([]byte, int) {
	if len(content) == 0 || n <= 0 {
		return nil, 0
	}

	lines := 0
	pos := len(content)

	for pos > 0 && lines < n {
		pos--
		if content[pos] == '\n' {
			lines++
		}
	}

	// Adjust to not include the newline we stopped at (unless at start).
	if pos > 0 {
		pos++
	}

	return content[pos:], lines
}

func extractLinesFromStart(content []byte, n int) ([]byte, int) {
	if len(content) == 0 || n <= 0 {
		return nil, 0
	}

	lines := 0
	pos := 0

	for pos < len(content) && lines < n {
		if content[pos] == '\n' {
			lines++
		}
		pos++
	}

	return content[:pos], lines
}

func joinBytes(chunks [][]byte) []byte {
	var size int
	for _, c := range chunks {
		size += len(c)
	}
	result := make([]byte, 0, size)
	for _, c := range chunks {
		result = append(result, c...)
	}
	return result
}
