package taskcenter

type markFlags struct {
	Ignored bool
	Handled bool
}

// markSet is keyed by taskType + "\x00" + sourceID
type markSet map[string]markFlags

func markKey(taskType, sourceID string) string {
	return taskType + "\x00" + sourceID
}

func applyMarks(dto *UnifiedTaskDTO, taskType, sourceID string, ms markSet) {
	if dto == nil {
		return
	}
	mf, ok := ms[markKey(taskType, sourceID)]
	if !ok {
		return
	}
	dto.Ignored = mf.Ignored
	dto.Handled = mf.Handled
}
