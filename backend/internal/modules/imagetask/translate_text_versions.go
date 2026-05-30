package imagetask

import (
	"strings"
)

func populateTranslationVersions(blocks []translateTextBlock, targetLang string) {
	for i := range blocks {
		populateBlockTranslationVersions(&blocks[i], targetLang)
	}
}

func populateBlockTranslationVersions(b *translateTextBlock, targetLang string) {
	if b == nil {
		return
	}
	literal := strings.TrimSpace(b.TranslatedText)
	if literal == "" {
		literal = strings.TrimSpace(b.Text)
	}
	if strings.TrimSpace(b.StandardTranslation) == "" {
		b.StandardTranslation = literal
	}
	if strings.TrimSpace(b.FixedShortTranslation) == "" {
		if v, ok := knownFixedShortTranslations[strings.TrimSpace(b.Text)]; ok {
			b.FixedShortTranslation = v
		}
	}
	short := strings.TrimSpace(b.ShortTranslatedText)
	if short == "" {
		short = ruleBasedShortText(b.Text, literal, targetLang)
	}
	b.ShortTranslatedText = short
	compact := strings.TrimSpace(b.CompactTranslation)
	if compact == "" {
		compact = ruleBasedCompactText(b.Text, literal, short, targetLang)
	}
	b.CompactTranslation = compact
	badge := strings.TrimSpace(b.BadgeTranslation)
	if badge == "" {
		badge = ruleBasedBadgeText(b.Text, compact, short, targetLang)
	}
	b.BadgeTranslation = badge
}

func ruleBasedCompactText(orig, literal, short, targetLang string) string {
	orig = strings.TrimSpace(orig)
	if v, ok := knownCompactTranslations[orig]; ok {
		return v
	}
	if short != "" && len([]rune(short)) <= len([]rune(literal)) {
		return short
	}
	return ruleBasedShortText(orig, literal, targetLang)
}

func ruleBasedBadgeText(orig, compact, short, targetLang string) string {
	orig = strings.TrimSpace(orig)
	if v, ok := knownBadgeTranslations[orig]; ok {
		return v
	}
	if compact != "" {
		return compact
	}
	return short
}

var knownFixedShortTranslations = map[string]string{
	"雪花白":        "Snow White",
	"炫酷黑":        "Cool Black",
	"折叠伸缩版/通用手机": "Universal Phone Stand",
	"折叠伸缩版／通用手机": "Universal Phone Stand",
	"折叠伸缩":       "Universal Stand",
	"通用手机":       "For Phones",
	"手机支架":       "Phone Stand",
}

var knownCompactTranslations = map[string]string{
	"雪花白":        "Snow White",
	"炫酷黑":        "Cool Black",
	"折叠伸缩版/通用手机": "Universal Phone Stand",
	"折叠伸缩版／通用手机": "Universal Phone Stand",
	"折叠伸缩":       "Universal Stand",
	"通用手机":       "For Phones",
	"手机支架":       "Phone Stand",
	"金属底座":       "Metal Base",
	"折叠支架":       "Foldable Stand",
	"手机/平板":      "Phone/Tablet",
	"手机 / 平板":    "Phone/Tablet",
	"暗夜黑":        "Midnight Black",
}

var knownBadgeTranslations = map[string]string{
	"雪花白":        "Snow White",
	"炫酷黑":        "Cool Black",
	"折叠伸缩版/通用手机": "Universal Stand",
	"折叠伸缩版／通用手机": "Universal Stand",
	"折叠伸缩":       "Universal Stand",
	"通用手机":       "For Phones",
	"手机支架":       "Phone Stand",
	"手机/平板":      "Phone/Tablet",
	"手机 / 平板":    "Phone/Tablet",
	"暗夜黑":        "Midnight Black",
}

func selectDrawTextForBlock(b translateTextBlock) string {
	if s := strings.TrimSpace(b.DrawText); s != "" {
		return s
	}
	return selectTranslationVersion(b, blockClassSmallCaption, 999, 1)
}

func selectTranslationVersion(b translateTextBlock, blockClass string, boxWidth, maxLines int) string {
	isBadge := blockClass == blockClassBadge || blockClass == blockClassColorBadge ||
		isCapsuleBlockClassForRender(blockClass) ||
		strings.Contains(strings.TrimSpace(b.Text), "/") || strings.Contains(strings.TrimSpace(b.Text), "／")
	candidates := []struct {
		text  string
		order int
	}{
		{strings.TrimSpace(b.FixedShortTranslation), 0},
		{strings.TrimSpace(b.BadgeTranslation), 1},
		{strings.TrimSpace(b.CompactTranslation), 2},
		{strings.TrimSpace(b.ShortTranslatedText), 3},
		{strings.TrimSpace(b.StandardTranslation), 4},
		{strings.TrimSpace(b.TranslatedText), 5},
	}
	if !isBadge {
		candidates[1], candidates[2] = candidates[2], candidates[1]
	}
	for _, c := range candidates {
		if c.text == "" {
			continue
		}
		if boxWidth <= 0 {
			return c.text
		}
		fs := 22
		if blockClass == blockClassTitle {
			fs = 28
		}
		if measureTextWidth(c.text, fs, false) <= float64(boxWidth)*1.02 {
			return c.text
		}
		if maxLines > 1 {
			lines := wrapTextToWidth(c.text, float64(boxWidth), fs, false, maxLines)
			if len(lines) > 0 && len(lines) <= maxLines {
				joined := strings.Join(lines, " ")
				if measureTextWidth(joined, fs, false) <= float64(boxWidth)*float64(maxLines)*1.05 {
					return c.text
				}
			}
		}
	}
	for _, c := range candidates {
		if c.text != "" {
			return c.text
		}
	}
	return ""
}
