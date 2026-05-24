package pdf417ech

import (
	"fmt"

	"github.com/boombuler/barcode"
)

const (
	macroMarker     = 928
	macroTerminator = 922
	macroOptional   = 923
)

type MacroSegment struct {
	FileID       string
	FileIDWords  []int
	FileName     string
	SegmentIndex int
	SegmentCount int
	Data         []byte
	Last         bool
}

func EncodeMacroSegment(segment MacroSegment) (barcode.Barcode, error) {
	words, err := macroDataWords(segment)
	if err != nil {
		return nil, err
	}
	return encodeCodewords(segment.FileID, words, echECLevel, barcode.ColorScheme16)
}

func MaxSegmentBytes(fileID, fileName string, segmentIndex, segmentCount int, last bool) int {
	low, high := 0, 512
	for {
		_, err := macroDataWords(MacroSegment{
			FileID:       fileID,
			FileName:     fileName,
			SegmentIndex: segmentIndex,
			SegmentCount: segmentCount,
			Data:         make([]byte, high),
			Last:         last,
		})
		if err != nil {
			break
		}
		low = high
		high *= 2
	}
	for low+1 < high {
		mid := (low + high) / 2
		_, err := macroDataWords(MacroSegment{
			FileID:       fileID,
			FileName:     fileName,
			SegmentIndex: segmentIndex,
			SegmentCount: segmentCount,
			Data:         make([]byte, mid),
			Last:         last,
		})
		if err == nil {
			low = mid
		} else {
			high = mid
		}
	}
	return low
}

func macroDataWords(segment MacroSegment) ([]int, error) {
	if segment.FileID == "" {
		return nil, fmt.Errorf("macro PDF417 file id is required")
	}
	if segment.SegmentIndex < 0 || segment.SegmentIndex > 99998 {
		return nil, fmt.Errorf("macro PDF417 segment index %d outside 0..99998", segment.SegmentIndex)
	}
	if segment.SegmentCount <= 0 || segment.SegmentCount > 99999 {
		return nil, fmt.Errorf("macro PDF417 segment count %d outside 1..99999", segment.SegmentCount)
	}

	payloadWords := encodeBinary(segment.Data, encText)
	controlWords, err := macroControlWords(segment)
	if err != nil {
		return nil, err
	}

	ecCount := securitylevel(echECLevel).ErrorCorrectionWordCount()
	capacity := echColumns*echRows - 1 - ecCount
	if len(payloadWords)+len(controlWords) > capacity {
		return nil, fmt.Errorf("macro PDF417 segment has %d data codewords; eCH capacity is %d", len(payloadWords)+len(controlWords), capacity)
	}
	padWords := make([]int, capacity-len(payloadWords)-len(controlWords))
	for i := range padWords {
		padWords[i] = padding_codeword
	}

	words := append(payloadWords, padWords...)
	words = append(words, controlWords...)
	return words, nil
}

func macroControlWords(segment MacroSegment) ([]int, error) {
	segmentIndexWords, err := encodeNumeric([]rune(fmt.Sprintf("%05d", segment.SegmentIndex)))
	if err != nil {
		return nil, fmt.Errorf("encoding macro segment index: %w", err)
	}
	words := []int{macroMarker}
	words = append(words, segmentIndexWords...)

	fileIDWords := append([]int(nil), segment.FileIDWords...)
	if len(fileIDWords) == 0 {
		fileIDWords, err = fileIDCodewords(segment.FileID)
		if err != nil {
			return nil, fmt.Errorf("encoding macro file id: %w", err)
		}
	}
	words = append(words, fileIDWords...)

	segmentCountWords, err := encodeNumeric([]rune(fmt.Sprintf("%05d", segment.SegmentCount)))
	if err != nil {
		return nil, fmt.Errorf("encoding macro segment count: %w", err)
	}
	words = append(words, macroOptional, 1)
	words = append(words, segmentCountWords...)

	if segment.FileName != "" {
		words = append(words, macroOptional, 0)
		words = append(words, encodeTextOnly(segment.FileName)...)
	}

	if segment.Last {
		words = append(words, macroTerminator)
	}

	return words, nil
}

func fileIDCodewords(fileID string) ([]int, error) {
	if fileID == "" {
		return nil, fmt.Errorf("macro PDF417 file id is required")
	}
	if len(fileID)%3 != 0 {
		return nil, fmt.Errorf("macro PDF417 file id %q length is not a multiple of 3", fileID)
	}
	words := make([]int, 0, len(fileID)/3)
	for i := 0; i < len(fileID); i += 3 {
		var word int
		if _, err := fmt.Sscanf(fileID[i:i+3], "%03d", &word); err != nil {
			return nil, err
		}
		if word < 0 || word > 899 {
			return nil, fmt.Errorf("macro PDF417 file id codeword %d outside 0..899", word)
		}
		words = append(words, word)
	}
	return words, nil
}

func encodeTextOnly(s string) []int {
	_, words := encodeText([]rune(s), subUpper)
	return words
}
