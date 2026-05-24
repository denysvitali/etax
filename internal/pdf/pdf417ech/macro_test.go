package pdf417ech

import "testing"

func TestMacroSegmentUsesFixedECHGeometry(t *testing.T) {
	code, err := EncodeMacroSegment(MacroSegment{
		FileID:       "123456234111",
		FileName:     "doc-1",
		SegmentIndex: 0,
		SegmentCount: 1,
		Data:         []byte("payload"),
		Last:         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code.Bounds().Dx() != 290 || code.Bounds().Dy() != 35 {
		t.Fatalf("bounds = %dx%d, want 290x35", code.Bounds().Dx(), code.Bounds().Dy())
	}
}

func TestMacroSegmentIncludesControlBlock(t *testing.T) {
	words, err := macroDataWords(MacroSegment{
		FileID:       "123456234111",
		FileName:     "doc-1",
		SegmentIndex: 0,
		SegmentCount: 1,
		Data:         []byte("payload"),
		Last:         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(words, macroMarker) {
		t.Fatalf("macro marker %d not found in %v", macroMarker, words)
	}
	if !contains(words, macroOptional) {
		t.Fatalf("macro optional marker %d not found in %v", macroOptional, words)
	}
	if words[len(words)-1] != macroTerminator {
		t.Fatalf("last codeword = %d, want macro terminator %d", words[len(words)-1], macroTerminator)
	}
}

func contains(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
