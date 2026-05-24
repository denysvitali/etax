package barcode

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/binarizer"
	"github.com/ericlevine/zxinggo/pdf417"
)

// DecodePDF417 decodes PDF417 symbols from a clean barcode image.
func DecodePDF417(r io.Reader) ([]string, error) {
	results, err := DecodePDF417Bytes(r)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(results))
	for i, result := range results {
		out[i] = string(result)
	}
	return out, nil
}

// DecodePDF417Bytes decodes PDF417 symbols from a clean barcode image.
func DecodePDF417Bytes(r io.Reader) ([][]byte, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}

	source := zxinggo.NewImageLuminanceSource(img)
	bitmaps := []*zxinggo.BinaryBitmap{
		zxinggo.NewBinaryBitmap(binarizer.NewGlobalHistogram(source)),
		zxinggo.NewBinaryBitmap(binarizer.NewHybrid(source)),
	}

	reader := pdf417.NewPDF417Reader()
	var decoded [][]byte
	seen := map[string]bool{}
	for _, bitmap := range bitmaps {
		for _, pure := range []bool{true, false} {
			results, err := reader.DecodeMultiple(bitmap, &zxinggo.DecodeOptions{
				PossibleFormats: []zxinggo.Format{zxinggo.FormatPDF417},
				TryHarder:       true,
				PureBarcode:     pure,
			})
			if err != nil {
				continue
			}
			for _, result := range results {
				if result == nil || result.Text == "" || seen[result.Text] {
					continue
				}
				seen[result.Text] = true
				decoded = append(decoded, latin1Bytes(result.Text))
			}
		}
	}
	if len(decoded) == 0 {
		return nil, fmt.Errorf("no PDF417 barcode found")
	}
	return decoded, nil
}

func latin1Bytes(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r <= 0xff {
			out = append(out, byte(r))
			continue
		}
		out = append(out, []byte(string(r))...)
	}
	return out
}
