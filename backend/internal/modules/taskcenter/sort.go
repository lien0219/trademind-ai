package taskcenter

import "sort"

func sortUnifiedDesc(xs []UnifiedTaskDTO) {
	sort.Slice(xs, func(i, j int) bool {
		ti := xs[i].SortKey
		tj := xs[j].SortKey
		if ti.Equal(tj) {
			return xs[i].ID > xs[j].ID
		}
		return ti.After(tj)
	})
}
