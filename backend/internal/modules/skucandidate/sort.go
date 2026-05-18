package skucandidate

import (
	"sort"
	"strconv"
	"strings"
)

func sortAndTrimCandidates(list []CandidateDTO, limit int) []CandidateDTO {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i].Confidence, list[j].Confidence
		if a != b {
			return a > b
		}
		return list[i].ProductSKUID < list[j].ProductSKUID
	})
	if limit > 0 && len(list) > limit {
		return list[:limit]
	}
	return list
}

func atoiCandidateQuery(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
