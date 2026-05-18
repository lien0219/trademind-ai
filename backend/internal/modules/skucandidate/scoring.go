package skucandidate

import "strings"

// mergeConfidence applies rule: capped at 95 unless external_sku_id_exact (100).
func mergeConfidence(scoreMap map[string]int, signals []string) (int, bool) {
	hasExtExact := containsStr(signals, "external_sku_id_equal")

	strongKeys := []string{"history_manual_bind", "publication_external", "publication_sku_exact", "publication_sku_norm", "local_sku_exact", "local_sku_norm"}
	weakSum := 0
	weakN := 0
	strongBest := 0
	for _, k := range strongKeys {
		if v, ok := scoreMap[k]; ok && v > strongBest {
			strongBest = v
		}
	}
	if v, ok := scoreMap["title"]; ok && v > 0 {
		weakSum += v
		weakN++
	}
	if v, ok := scoreMap["attrs"]; ok && v > 0 {
		weakSum += v
		weakN++
	}

	if hasExtExact || strongBest >= 100 {
		return 100, hasExtExact
	}
	if weakN == 0 {
		conf := strongBest
		if conf <= 0 {
			return 0, false
		}
		if conf > 95 {
			conf = 95
		}
		return conf, false
	}

	weakAvg := weakSum / weakN
	if strongBest <= 0 {
		c := weakAvg
		if c > 95 {
			c = 95
		}
		return c, false
	}

	boost := weakAvg / 6
	conf := strongBest + boost
	if conf > 95 {
		conf = 95
	}
	return conf, false
}

func containsStr(slice []string, want string) bool {
	for _, s := range slice {
		if s == want {
			return true
		}
	}
	return false
}

func primarySource(scoreMap map[string]int) CandidateSource {
	order := []struct {
		key string
		src CandidateSource
	}{
		{"history_manual_bind", SourceHistoryManualBind},
		{"publication_external", SourcePublicationMapping},
		{"publication_sku_exact", SourcePublicationMapping},
		{"publication_sku_norm", SourcePublicationMapping},
		{"local_sku_exact", SourceLocalSKUCode},
		{"local_sku_norm", SourceLocalSKUCode},
		{"title", SourceTitleSimilarity},
		{"attrs", SourceAttrsSimilarity},
	}
	best := -1
	var picked CandidateSource = SourceAttrsSimilarity
	for _, row := range order {
		v, ok := scoreMap[row.key]
		if !ok || v <= 0 {
			continue
		}
		if v > best {
			best = v
			picked = row.src
		}
	}
	return picked
}

func buildReason(primary CandidateSource, conf int, sigs []string) string {
	var b strings.Builder
	b.WriteString(string(primary))
	b.WriteString(" · ")
	switch {
	case conf >= 90:
		b.WriteString("高分匹配")
	case conf >= 70:
		b.WriteString("中分匹配")
	case conf >= defaultMinConfidence:
		b.WriteString("低分参考")
	default:
		b.WriteString("弱信号参考")
	}
	if len(sigs) > 0 {
		b.WriteString(" · ")
		for i, s := range sigs {
			if i > 0 {
				b.WriteString("+")
			}
			b.WriteString(s)
			if i >= 7 {
				b.WriteString("…")
				break
			}
		}
	}
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(b.String(), "__", "_"), "..", "."))
}
