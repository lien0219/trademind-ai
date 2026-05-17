package productpublish

import (
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type publishSnapshot struct {
	PublicationID uuid.UUID         `json:"publicationId"`
	MergedPublish map[string]string `json:"mergedPublish"`
	Options       map[string]any    `json:"options"`
}

func parsePublishSnapshot(raw datatypes.JSON) (publishSnapshot, error) {
	var snap publishSnapshot
	if len(raw) == 0 {
		return snap, errors.New("empty task input")
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		return publishSnapshot{}, err
	}
	if snap.PublicationID == uuid.Nil {
		return publishSnapshot{}, errors.New("snapshot missing publicationId")
	}
	if snap.MergedPublish == nil {
		snap.MergedPublish = map[string]string{}
	}
	return snap, nil
}

func snapshotPublicationFromTask(task *ProductPublishTask) (uuid.UUID, bool) {
	if task == nil {
		return uuid.Nil, false
	}
	ps, err := parsePublishSnapshot(task.Input)
	if err != nil || ps.PublicationID == uuid.Nil {
		return uuid.Nil, false
	}
	return ps.PublicationID, true
}
