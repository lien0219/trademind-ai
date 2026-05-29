package imagetask

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/aimodelparse"
)

const layoutWarningBBoxRepaired = "ocr_bbox_repaired"

const defaultTranslateTextColor = "#111111"

func needsOCRBBoxRepair(blocks []translateTextBlock) bool {
	if len(blocks) < 2 {
		return false
	}
	invalidY := 0
	for _, b := range blocks {
		if b.BBox.Y <= 2 {
			invalidY++
		}
	}
	if invalidY >= 2 {
		return true
	}
	for i := 0; i < len(blocks); i++ {
		for j := i + 1; j < len(blocks); j++ {
			if bboxOverlapAreaRatio(blocks[i].BBox, blocks[j].BBox) > 0.35 {
				return true
			}
		}
	}
	return false
}

func bboxOverlapAreaRatio(a, b translateTextBBox) float64 {
	if a.Width <= 0 || a.Height <= 0 || b.Width <= 0 || b.Height <= 0 {
		return 0
	}
	xOverlap := minInt(a.X+a.Width, b.X+b.Width) - maxInt(a.X, b.X)
	yOverlap := minInt(a.Y+a.Height, b.Y+b.Height) - maxInt(a.Y, b.Y)
	if xOverlap <= 0 || yOverlap <= 0 {
		return 0
	}
	overlap := float64(xOverlap * yOverlap)
	smaller := float64(minInt(a.Width*a.Height, b.Width*b.Height))
	if smaller <= 0 {
		return 0
	}
	return overlap / smaller
}

func (s *Service) repairOCRBlockBBoxes(
	ctx context.Context,
	imageRef string,
	blocks []translateTextBlock,
	imageW, imageH int,
) []translateTextBlock {
	if !needsOCRBBoxRepair(blocks) {
		return blocks
	}
	if refined := s.refineOCRBlockBBoxes(ctx, imageRef, blocks, imageW, imageH); len(refined) > 0 && !needsOCRBBoxRepair(refined) {
		return refined
	}
	return heuristicRepairOCRBlockBBoxes(blocks, imageW, imageH)
}

func (s *Service) refineOCRBlockBBoxes(
	ctx context.Context,
	imageRef string,
	blocks []translateTextBlock,
	imageW, imageH int,
) []translateTextBlock {
	if s == nil || len(blocks) == 0 || strings.TrimSpace(imageRef) == "" {
		return nil
	}
	var lines []string
	for i, b := range blocks {
		text := strings.TrimSpace(b.Text)
		if text == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf(`%d. %q`, i+1, text))
	}
	if len(lines) == 0 {
		return nil
	}
	sizeHint := ""
	if imageW > 0 && imageH > 0 {
		sizeHint = fmt.Sprintf("Image size: %d x %d pixels.\n", imageW, imageH)
	}
	prompt := fmt.Sprintf(`%sFor each text string below, locate the EXACT visible text region on this product image.
Return precise pixel bounding boxes (x, y, width, height) for each text overlay.
Do NOT merge separate lines into one box. y must reflect vertical position on the image (not 0 for all items).

Return ONLY JSON:
{"blocks":[{"index":1,"text":"original","bbox":{"x":0,"y":0,"width":100,"height":40}}]}

Texts:
%s`, sizeHint, strings.Join(lines, "\n"))

	content, err := s.chatVisionJSON(ctx, prompt, imageRef, 1800)
	if err != nil {
		return nil
	}
	return applyBBoxRefinement(blocks, content, imageW, imageH)
}

func applyBBoxRefinement(blocks []translateTextBlock, content string, imageW, imageH int) []translateTextBlock {
	normalized := aimodelparse.NormalizeJSONContent(content)
	if normalized == "" {
		return nil
	}
	var root struct {
		Blocks []struct {
			Index int    `json:"index"`
			Text  string `json:"text"`
			BBox  translateTextBBox
		} `json:"blocks"`
	}
	if err := json.Unmarshal([]byte(normalized), &root); err != nil {
		flex, flexErr := parseBBoxRefinementFlexible(normalized)
		if flexErr != nil {
			return nil
		}
		root.Blocks = flex
	}
	if len(root.Blocks) == 0 {
		return nil
	}

	out := append([]translateTextBlock{}, blocks...)
	updated := 0
	for _, item := range root.Blocks {
		bb := normalizeRefinedBBox(item.BBox, imageW, imageH)
		if bb.Width <= 0 || bb.Height <= 0 {
			continue
		}
		idx := item.Index - 1
		if idx < 0 || idx >= len(out) {
			if t := strings.TrimSpace(item.Text); t != "" {
				for i := range out {
					if strings.EqualFold(strings.TrimSpace(out[i].Text), t) {
						idx = i
						break
					}
				}
			}
		}
		if idx < 0 || idx >= len(out) {
			continue
		}
		out[idx].BBox = bb
		updated++
	}
	if updated == 0 {
		return nil
	}
	return out
}

func parseBBoxRefinementFlexible(content string) ([]struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
	BBox  translateTextBBox
}, error) {
	var root map[string]any
	if err := json.Unmarshal([]byte(content), &root); err != nil {
		return nil, err
	}
	raw := firstOCRAny(root, "blocks", "items", "regions")
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("no blocks")
	}
	var out []struct {
		Index int    `json:"index"`
		Text  string `json:"text"`
		BBox  translateTextBBox
	}
	for i, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		text := firstOCRString(m, "text", "original", "original_text")
		idx := i + 1
		if v, ok := m["index"].(float64); ok && int(v) > 0 {
			idx = int(v)
		}
		bb := parseOCRBBox(firstOCRAny(m, "bbox", "bounding_box", "boundingBox", "box"))
		if bb.Width <= 0 || bb.Height <= 0 {
			continue
		}
		out = append(out, struct {
			Index int    `json:"index"`
			Text  string `json:"text"`
			BBox  translateTextBBox
		}{Index: idx, Text: text, BBox: bb})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no refined blocks")
	}
	return out, nil
}

func normalizeRefinedBBox(bb translateTextBBox, imageW, imageH int) translateTextBBox {
	x, y, w, h := bb.X, bb.Y, bb.Width, bb.Height
	if imageW > 0 || imageH > 0 {
		x, y, w, h = clampTranslateBBox(x, y, w, h, imageW, imageH)
	}
	bb.X, bb.Y, bb.Width, bb.Height = x, y, w, h
	if imageH > 0 && bb.Height > imageH/3 {
		bb.Height = estimateSingleLineBBoxHeight("", imageH)
		if bb.Y+bb.Height > imageH {
			bb.Y = maxInt(0, imageH-bb.Height)
		}
	}
	return bb
}

func clampTranslateBBox(x, y, w, h, imageW, imageH int) (int, int, int, int) {
	minH := 24
	minW := 40
	if w < minW {
		w = minW
	}
	if h < minH {
		h = minH
	}
	if imageW > 0 {
		if x < 0 {
			x = 0
		}
		if x+w > imageW {
			if w > imageW {
				w = imageW
				x = 0
			} else {
				x = imageW - w
			}
		}
		if x < 0 {
			x = 0
		}
	} else if x < 0 {
		x = 0
	}
	if imageH > 0 {
		if y < 0 {
			y = 0
		}
		if y+h > imageH {
			y = imageH - h
		}
		if y < 0 {
			y = 0
			if h > imageH {
				h = imageH
			}
		}
	} else if y < 0 {
		y = 0
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return x, y, w, h
}

func heuristicRepairOCRBlockBBoxes(blocks []translateTextBlock, imageW, imageH int) []translateTextBlock {
	if len(blocks) == 0 || imageH <= 0 {
		return blocks
	}
	out := append([]translateTextBlock{}, blocks...)
	startY := maxInt(int(float64(imageH)*0.04), 8)
	gap := maxInt(int(float64(imageH)*0.015), 4)
	curY := startY
	defaultW := imageW * 55 / 100
	if defaultW <= 0 {
		defaultW = 240
	}
	defaultX := maxInt(imageW*4/100, 8)
	for i := range out {
		h := out[i].BBox.Height
		if h <= 0 || h > imageH/4 {
			h = estimateSingleLineBBoxHeight(out[i].Text, imageH)
		}
		w := out[i].BBox.Width
		if w <= 0 || w > imageW*9/10 {
			w = defaultW
		}
		x := out[i].BBox.X
		if x <= 0 {
			x = defaultX
		}
		out[i].BBox = translateTextBBox{X: x, Y: curY, Width: w, Height: h}
		curY += h + gap
	}
	return out
}

func estimateSingleLineBBoxHeight(text string, imageH int) int {
	fs := 56
	switch {
	case imageH > 0 && imageH < 600:
		fs = 42
	case imageH >= 900:
		fs = 64
	}
	if len([]rune(strings.TrimSpace(text))) > 8 {
		fs = fs * 9 / 10
	}
	return fs + 10
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clampOCRBlockBBoxes(blocks []translateTextBlock, imageW, imageH int) []translateTextBlock {
	if len(blocks) == 0 || (imageW <= 0 && imageH <= 0) {
		return blocks
	}
	out := append([]translateTextBlock{}, blocks...)
	for i := range out {
		minH := estimateSingleLineBBoxHeight(out[i].Text, imageH)
		if minH < 24 {
			minH = 24
		}
		w := out[i].BBox.Width
		h := out[i].BBox.Height
		if h < minH {
			h = minH
		}
		if w < 40 {
			w = 40
		}
		x, y, w, h := clampTranslateBBox(out[i].BBox.X, out[i].BBox.Y, w, h, imageW, imageH)
		if h < minH {
			h = minH
			if imageH > 0 && y+h > imageH {
				y = imageH - h
				if y < 0 {
					y = 0
				}
			}
		}
		out[i].BBox = translateTextBBox{X: x, Y: y, Width: w, Height: h}
	}
	return out
}
