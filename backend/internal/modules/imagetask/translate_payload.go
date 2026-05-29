package imagetask

import (
	"fmt"
	"strings"
)

func decodePayloadBytes(payload *translateImagePayload) ([]byte, error) {
	if payload == nil || len(payload.RawBytes) == 0 {
		return nil, fmt.Errorf("empty image payload")
	}
	return payload.RawBytes, nil
}

func inferBlockStyles(payload *translateImagePayload, blocks []translateTextBlock) {
	for i := range blocks {
		if blocks[i].Style.Align == "" {
			blocks[i].Style.Align = "left"
		}
		if strings.TrimSpace(blocks[i].Style.Color) == "" {
			blocks[i].Style.Color = defaultTranslateTextColor
		}
	}
}
