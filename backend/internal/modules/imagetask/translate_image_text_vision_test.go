package imagetask

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"testing"
)

func TestPayloadFromDataURL(t *testing.T) {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	payload, err := payloadFromDataURL("data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()))
	if err != nil {
		t.Fatalf("payloadFromDataURL: %v", err)
	}
	if len(payload.RawBytes) == 0 || payload.Width != 8 || payload.Height != 8 {
		t.Fatalf("unexpected payload: bytes=%d w=%d h=%d", len(payload.RawBytes), payload.Width, payload.Height)
	}
}

func TestLoadTranslateImagePayloadDataURL(t *testing.T) {
	s := &Service{}
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	payload, err := s.loadTranslateImagePayload(t.Context(), dataURL)
	if err != nil {
		t.Fatalf("loadTranslateImagePayload: %v", err)
	}
	if payload == nil || len(payload.RawBytes) == 0 {
		t.Fatal("expected payload bytes")
	}
}

func TestVerifyAllowsSkippedReOCRWhenImageChanged(t *testing.T) {
	s := &Service{}
	src := []byte{1, 2, 3, 4}
	out := []byte{1, 2, 3, 5}
	ocr := &translateOCRResult{
		Blocks: []translateTextBlock{
			{Text: "金属底座", TranslatedText: "Metal Base"},
		},
	}
	meta, err := s.verifyTranslateOutput(t.Context(), src, out, ocr, "en", "zh", true)
	if err != nil {
		t.Fatalf("expected success when image changed and blocks rendered, got %v", err)
	}
	if !meta.OutputTextVerifySkipped || !meta.TargetTextDetected {
		t.Fatalf("unexpected meta: %+v", meta)
	}
}
