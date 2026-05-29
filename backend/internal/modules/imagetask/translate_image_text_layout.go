package imagetask

import (
	"encoding/json"
	"math"
	"strings"
	"unicode"
)

type translateLayoutOptions struct {
	AutoLayout         bool
	AutoWrap           bool
	AutoFontSize       bool
	AllowTextBoxExpand bool
	AllowTextSimplify  bool
	MinFontSize        int
	MaxFontSize        int
	LineHeightRatio    float64
	MaxLines           int
	LayoutMode         string
	TargetLang         string
}

type translateBlockLayoutPlan struct {
	DisplayText   string
	Lines         []string
	FontSize      int
	BBox          translateTextBBox
	Wrapped       bool
	FontResized   bool
	Simplified    bool
	Overflow      bool
	Expanded      bool
	UsedShortText bool
}

type translateLayoutSummary struct {
	AutoLayout        bool     `json:"autoLayout"`
	TextBlocksCount   int      `json:"textBlocksCount"`
	AutoWrappedBlocks int      `json:"autoWrappedBlocks"`
	FontResizedBlocks int      `json:"fontResizedBlocks"`
	SimplifiedBlocks  int      `json:"simplifiedBlocks"`
	OverflowBlocks    int      `json:"overflowBlocks"`
	MinFontSizeUsed   int      `json:"minFontSizeUsed"`
	Warnings          []string `json:"warnings"`
}

const (
	layoutWarningTextTooLong  = "translated_text_too_long"
	layoutWarningFontAdjusted = "font_size_auto_adjusted"
	layoutWarningSimplified   = "translated_text_simplified"
	layoutWarningOverflow     = "translated_text_overflow"
	longTextRatioThreshold    = 1.8
	maxBBoxExpandRatio        = 0.30
)

func floatFromAny(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	default:
		return 0
	}
}

func parseTranslateLayoutOptions(hints map[string]any, targetLang string) translateLayoutOptions {
	mode := strings.TrimSpace(strings.ToLower(stringFromMap(hints, "layoutMode")))
	opts := translateLayoutOptions{
		AutoLayout:         boolFromHints(hints, "autoLayout", true),
		AutoWrap:           boolFromHints(hints, "autoWrap", true),
		AutoFontSize:       boolFromHints(hints, "autoFontSize", true),
		AllowTextBoxExpand: boolFromHints(hints, "allowTextBoxExpand", true),
		AllowTextSimplify:  boolFromHints(hints, "allowTextSimplify", true),
		MinFontSize:        intFromAny(hints["minFontSize"]),
		MaxFontSize:        intFromAny(hints["maxFontSize"]),
		LineHeightRatio:    floatFromAny(hints["lineHeightRatio"]),
		MaxLines:           intFromAny(hints["maxLines"]),
		LayoutMode:         mode,
		TargetLang:         targetLang,
	}
	if opts.MinFontSize <= 0 {
		opts.MinFontSize = 14
	}
	if opts.MaxFontSize <= 0 {
		opts.MaxFontSize = 52
	}
	if opts.LineHeightRatio <= 0 {
		opts.LineHeightRatio = 1.15
	}
	if opts.MaxLines <= 0 {
		opts.MaxLines = 3
	}
	switch mode {
	case "preserve":
		if hints == nil || hints["allowTextBoxExpand"] == nil {
			opts.AllowTextBoxExpand = false
		}
		if hints == nil || hints["allowTextSimplify"] == nil {
			opts.AllowTextSimplify = false
		}
	case "readable":
		opts.AllowTextSimplify = boolFromHints(hints, "allowTextSimplify", true)
		if opts.MinFontSize < 16 {
			opts.MinFontSize = 16
		}
		if opts.MaxLines < 4 {
			opts.MaxLines = 4
		}
	}
	return opts
}

func isCJKLang(code string) bool {
	switch strings.TrimSpace(strings.ToLower(code)) {
	case "zh", "cn", "chinese", "ja", "ko":
		return true
	default:
		return false
	}
}

func isCJKRune(r rune) bool {
	return unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r)
}

func mostlyCJK(text string) bool {
	var cjk, total int
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		total++
		if isCJKRune(r) {
			cjk++
		}
	}
	return total > 0 && float64(cjk)/float64(total) >= 0.5
}

func estimateTextWidth(text string, fontSize int, cjk bool) float64 {
	if fontSize <= 0 {
		fontSize = 14
	}
	var w float64
	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			w += float64(fontSize) * 0.28
		case cjk || isCJKRune(r):
			w += float64(fontSize) * 1.0
		default:
			w += float64(fontSize) * 0.55
		}
	}
	return w
}

func lineBlockHeight(lineCount, fontSize int, ratio float64) float64 {
	if lineCount <= 0 {
		return 0
	}
	if ratio <= 0 {
		ratio = 1.15
	}
	return float64(lineCount) * float64(fontSize) * ratio
}

func estimateInitialFontSize(bbox translateTextBBox, text string, opts translateLayoutOptions) int {
	h := bbox.Height
	if h <= 0 {
		h = 36
	}
	fs := int(math.Floor(float64(h) / opts.LineHeightRatio))
	if bbox.Width > 0 {
		runes := []rune(strings.TrimSpace(text))
		if len(runes) > 0 {
			cjk := isCJKLang(opts.TargetLang) || mostlyCJK(text)
			factor := 0.55
			if cjk {
				factor = 1.0
			}
			fsW := int(math.Floor(float64(bbox.Width) / (float64(len(runes)) * factor)))
			if fsW > 0 && (fs == 0 || fsW < fs) {
				fs = fsW
			}
		}
	}
	if fs > opts.MaxFontSize {
		fs = opts.MaxFontSize
	}
	if fs < opts.MinFontSize {
		fs = opts.MinFontSize
	}
	return fs
}

func wrapTextToWidth(text string, maxWidth float64, fontSize int, cjk bool, maxLines int) []string {
	text = strings.TrimSpace(text)
	if text == "" || maxWidth <= 0 {
		return nil
	}
	if maxLines <= 0 {
		maxLines = 3
	}
	var lines []string
	if cjk {
		lines = wrapCJKText(text, maxWidth, fontSize)
	} else {
		lines = wrapEnglishWords(text, maxWidth, fontSize)
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return lines
}

func wrapEnglishWords(text string, maxWidth float64, fontSize int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	var current strings.Builder
	for _, word := range words {
		trial := word
		if current.Len() > 0 {
			trial = current.String() + " " + word
		}
		if estimateTextWidth(trial, fontSize, false) <= maxWidth {
			if current.Len() > 0 {
				current.WriteByte(' ')
			}
			current.WriteString(word)
			continue
		}
		if current.Len() > 0 {
			lines = append(lines, current.String())
			current.Reset()
		}
		if estimateTextWidth(word, fontSize, false) <= maxWidth {
			current.WriteString(word)
		} else {
			lines = append(lines, word)
		}
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return lines
}

func wrapCJKText(text string, maxWidth float64, fontSize int) []string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return nil
	}
	var lines []string
	var current []rune
	for _, r := range runes {
		trial := string(append(current, r))
		if estimateTextWidth(trial, fontSize, true) <= maxWidth {
			current = append(current, r)
			continue
		}
		if len(current) > 0 {
			lines = append(lines, string(current))
			current = []rune{r}
		} else {
			lines = append(lines, string(r))
			current = nil
		}
	}
	if len(current) > 0 {
		lines = append(lines, string(current))
	}
	return fixCJKLinePunctuation(lines)
}

func fixCJKLinePunctuation(lines []string) []string {
	if len(lines) <= 1 {
		return lines
	}
	punct := "，。！？、；：）】》」』\"'"
	for i := 1; i < len(lines); i++ {
		runes := []rune(strings.TrimSpace(lines[i]))
		if len(runes) == 0 {
			continue
		}
		if strings.ContainsRune(punct, runes[0]) {
			prev := []rune(lines[i-1])
			lines[i-1] = string(append(prev, runes[0]))
			if len(runes) > 1 {
				lines[i] = string(runes[1:])
			} else {
				lines[i] = ""
			}
		}
	}
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func layoutFits(lines []string, fontSize int, bbox translateTextBBox, opts translateLayoutOptions) bool {
	if len(lines) == 0 {
		return true
	}
	cjk := mostlyCJK(strings.Join(lines, ""))
	maxW := 0.0
	for _, ln := range lines {
		w := estimateTextWidth(ln, fontSize, cjk)
		if w > maxW {
			maxW = w
		}
	}
	h := lineBlockHeight(len(lines), fontSize, opts.LineHeightRatio)
	widthOK := bbox.Width <= 0 || maxW <= float64(bbox.Width)+1
	heightOK := bbox.Height <= 0 || h <= float64(bbox.Height)+1
	return widthOK && heightOK
}

func expandBBox(bbox translateTextBBox, imageW, imageH int) translateTextBBox {
	if bbox.Width <= 0 || bbox.Height <= 0 {
		return bbox
	}
	extraW := int(math.Floor(float64(bbox.Width) * maxBBoxExpandRatio))
	extraH := int(math.Floor(float64(bbox.Height) * maxBBoxExpandRatio))
	out := bbox
	out.X -= extraW / 2
	out.Y -= extraH / 2
	out.Width += extraW
	out.Height += extraH
	if out.X < 0 {
		out.X = 0
	}
	if out.Y < 0 {
		out.Y = 0
	}
	if imageW > 0 && out.X+out.Width > imageW {
		out.Width = imageW - out.X
	}
	if imageH > 0 && out.Y+out.Height > imageH {
		out.Height = imageH - out.Y
	}
	return out
}

func truncateLinesToFit(lines []string, fontSize int, bbox translateTextBBox, opts translateLayoutOptions) ([]string, bool) {
	if len(lines) == 0 {
		return lines, false
	}
	cjk := mostlyCJK(strings.Join(lines, ""))
	out := make([]string, len(lines))
	copy(out, lines)
	overflow := false
	for len(out) > opts.MaxLines {
		out = out[:opts.MaxLines]
		overflow = true
	}
	for !layoutFits(out, fontSize, bbox, opts) && len(out) > 0 {
		last := out[len(out)-1]
		runes := []rune(last)
		if len(runes) <= 1 {
			out = out[:len(out)-1]
			overflow = true
			continue
		}
		out[len(out)-1] = string(runes[:len(runes)-1]) + "…"
		overflow = true
		if estimateTextWidth(out[len(out)-1], fontSize, cjk) <= float64(bbox.Width)+1 {
			break
		}
	}
	if !layoutFits(out, fontSize, bbox, opts) {
		overflow = true
	}
	return out, overflow
}

func layoutTranslateBlock(text, shortText string, bbox translateTextBBox, opts translateLayoutOptions, imageW, imageH int) translateBlockLayoutPlan {
	plan := translateBlockLayoutPlan{
		DisplayText: strings.TrimSpace(text),
		BBox:        bbox,
		FontSize:    estimateInitialFontSize(bbox, text, opts),
	}
	if plan.DisplayText == "" {
		return plan
	}
	if !opts.AutoLayout {
		plan.Lines = []string{plan.DisplayText}
		return plan
	}

	cjk := isCJKLang(opts.TargetLang) || mostlyCJK(plan.DisplayText)
	origLen := len([]rune(plan.DisplayText))
	short := strings.TrimSpace(shortText)
	tryShortFirst := short != "" && short != plan.DisplayText && opts.AllowTextSimplify

	attempt := func(display string, useShort bool) translateBlockLayoutPlan {
		p := plan
		p.DisplayText = display
		p.UsedShortText = useShort
		p.Simplified = useShort
		workBBox := bbox
		fontSize := estimateInitialFontSize(workBBox, display, opts)
		initialFS := fontSize
		var lines []string
		singleLine := display
		if estimateTextWidth(singleLine, fontSize, cjk) <= float64(workBBox.Width)+1 || !opts.AutoWrap {
			lines = []string{singleLine}
		} else {
			lines = wrapTextToWidth(display, float64(workBBox.Width), fontSize, cjk, opts.MaxLines)
			if len(lines) > 1 {
				p.Wrapped = true
			}
		}

		if opts.AutoFontSize {
			for !layoutFits(lines, fontSize, workBBox, opts) && fontSize > opts.MinFontSize {
				fontSize--
				if opts.AutoWrap {
					lines = wrapTextToWidth(display, float64(workBBox.Width), fontSize, cjk, opts.MaxLines)
				}
				if fontSize < initialFS {
					p.FontResized = true
				}
			}
		}

		if !layoutFits(lines, fontSize, workBBox, opts) && fontSize <= opts.MinFontSize && opts.AllowTextBoxExpand {
			expanded := expandBBox(workBBox, imageW, imageH)
			if expanded.Width != workBBox.Width || expanded.Height != workBBox.Height {
				workBBox = expanded
				p.Expanded = true
				if opts.AutoWrap {
					lines = wrapTextToWidth(display, float64(workBBox.Width), fontSize, cjk, opts.MaxLines)
				}
				if len(lines) > 1 {
					p.Wrapped = true
				}
			}
		}

		if !layoutFits(lines, fontSize, workBBox, opts) {
			lines, overflow := truncateLinesToFit(lines, fontSize, workBBox, opts)
			p.Overflow = overflow
			p.Lines = lines
			p.FontSize = fontSize
			p.BBox = workBBox
			return p
		}

		p.Lines = lines
		p.FontSize = fontSize
		p.BBox = workBBox
		return p
	}

	best := attempt(plan.DisplayText, false)
	if tryShortFirst {
		shortPlan := attempt(short, true)
		if !shortPlan.Overflow || (best.Overflow && !shortPlan.Overflow) {
			best = shortPlan
		} else if shortPlan.FontSize >= best.FontSize && len(shortPlan.Lines) <= len(best.Lines) {
			best = shortPlan
		}
	}

	trLen := len([]rune(best.DisplayText))
	if origLen > 0 && float64(trLen) > float64(origLen)*longTextRatioThreshold {
		// caller adds warning
		_ = trLen
	}
	return best
}

func computeTranslateLayouts(blocks []translateTextBlock, opts translateLayoutOptions, imageW, imageH int) ([]translateBlockLayoutPlan, translateLayoutSummary) {
	summary := translateLayoutSummary{
		AutoLayout:      opts.AutoLayout,
		TextBlocksCount: len(blocks),
		MinFontSizeUsed: opts.MaxFontSize,
		Warnings:        []string{},
	}
	if summary.MinFontSizeUsed <= 0 {
		summary.MinFontSizeUsed = opts.MaxFontSize
	}
	plans := make([]translateBlockLayoutPlan, 0, len(blocks))
	hasLongText := false
	hasFontAdjusted := false
	hasSimplified := false
	hasOverflow := false

	for _, b := range blocks {
		text := strings.TrimSpace(b.TranslatedText)
		if text == "" {
			continue
		}
		origLen := len([]rune(strings.TrimSpace(b.Text)))
		trLen := len([]rune(text))
		if origLen > 0 && float64(trLen) > float64(origLen)*longTextRatioThreshold {
			hasLongText = true
		}
		plan := layoutTranslateBlock(text, b.ShortTranslatedText, b.BBox, opts, imageW, imageH)
		if plan.Wrapped {
			summary.AutoWrappedBlocks++
		}
		if plan.FontResized {
			summary.FontResizedBlocks++
			hasFontAdjusted = true
		}
		if plan.Simplified || plan.UsedShortText {
			summary.SimplifiedBlocks++
			hasSimplified = true
		}
		if plan.Overflow {
			summary.OverflowBlocks++
			hasOverflow = true
		}
		if plan.FontSize > 0 && (summary.MinFontSizeUsed == 0 || plan.FontSize < summary.MinFontSizeUsed) {
			summary.MinFontSizeUsed = plan.FontSize
		}
		plans = append(plans, plan)
	}

	if hasLongText && !hasSimplified && !hasOverflow {
		summary.Warnings = append(summary.Warnings, layoutWarningTextTooLong)
	}
	if hasFontAdjusted {
		summary.Warnings = append(summary.Warnings, layoutWarningFontAdjusted)
	}
	if hasSimplified && !hasOverflow {
		summary.Warnings = append(summary.Warnings, layoutWarningSimplified)
	}
	if hasOverflow {
		summary.Warnings = append(summary.Warnings, layoutWarningOverflow)
	}
	if summary.MinFontSizeUsed <= 0 || summary.MinFontSizeUsed > opts.MaxFontSize {
		summary.MinFontSizeUsed = opts.MinFontSize
	}
	return plans, summary
}

func ruleBasedShortText(orig, translated, targetLang string) string {
	t := strings.TrimSpace(translated)
	if t == "" {
		t = strings.TrimSpace(orig)
	}
	if t == "" {
		return t
	}
	lower := strings.ToLower(t)
	known := map[string]string{
		"free shipping nationwide":         "Free Shipping",
		"free shipping across the country": "Free Shipping",
		"free shipping":                    "Free Shipping",
		"high quality durable material":    "Durable Quality",
		"high quality material":            "Quality Material",
		"nationwide free shipping":         "Free Shipping",
		"全国包邮":                             "Free Shipping",
		"包邮":                               "Free Ship",
		"高品质耐用材质":                          "Durable",
		"金属底座":                             "Metal Base",
		"折叠支架":                             "Foldable Stand",
		"手机/平板":                            "Phone/Tablet",
		"手机 / 平板":                          "Phone/Tablet",
		"暗夜黑":                              "Midnight Black",
		"金属底座 折叠支架":                        "Metal Folding Stand",
		"金属底座折叠支架":                         "Metal Folding Stand",
		"柔韧耐折 防滑减震":                        "Flexible Anti-Slip",
		"柔韧耐折防滑减震":                         "Flexible Anti-Slip",
		"超值实惠装100片":                        "100 pcs Value Pack",
		"新款十二生肖":                           "New Zodiac Series",
	}
	if v, ok := known[lower]; ok {
		return v
	}
	if v, ok := known[strings.TrimSpace(orig)]; ok {
		return v
	}
	if isCJKLang(targetLang) {
		runes := []rune(t)
		if len(runes) > 8 {
			return string(runes[:8])
		}
		return t
	}
	words := strings.Fields(t)
	if len(words) <= 2 {
		return t
	}
	short := strings.Join(words[:2], " ")
	if len(words) >= 3 && len(short) < len(t)/2 {
		return short
	}
	if len(words) > 3 {
		return strings.Join(words[:3], " ")
	}
	return t
}

func needsShortText(orig, translated string) bool {
	origLen := len([]rune(strings.TrimSpace(orig)))
	trLen := len([]rune(strings.TrimSpace(translated)))
	if origLen == 0 || trLen == 0 {
		return false
	}
	return float64(trLen) > float64(origLen)*longTextRatioThreshold
}
