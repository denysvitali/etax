package barcode

import (
	"bytes"
	"testing"
)

func TestLatin1BytesPreservesPDF417BinaryPayload(t *testing.T) {
	payload := []byte{0x78, 0xda, 0xec, 0xbd, 0x00, 0xff, 0x41}
	runes := make([]rune, len(payload))
	for i, b := range payload {
		runes[i] = rune(b)
	}
	if got := latin1Bytes(string(runes)); !bytes.Equal(got, payload) {
		t.Fatalf("latin1Bytes = % x, want % x", got, payload)
	}
}
