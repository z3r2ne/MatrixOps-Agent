package session

import (
	"fmt"

	"matrixops-agent/snapshot"
	"matrixops-agent/types"
	"matrixops-agent/util"
	"pkgs/db/models"
	"pkgs/db/storage"

	"gorm.io/gorm"
)

func (r *AgentRunner) emitFinishSnapshotPatch(runtimeConfig *RuntimeConfig, description string) error {
	if runtimeConfig == nil || runtimeConfig.Assistant == nil || runtimeConfig.Assistant.Snapshot == "" {
		return nil
	}
	if r.db == nil {
		return nil
	}

	finishSnapshot, err := snapshot.Track(r.GetProjectID(), r.GetDirectory())
	if err != nil {
		return fmt.Errorf("track finish snapshot: %w", err)
	}

	patch, err := snapshot.PatchFiles(r.GetProjectID(), r.GetDirectory(), runtimeConfig.Assistant.Snapshot)
	if err != nil {
		return fmt.Errorf("build finish snapshot patch: %w", err)
	}
	if len(patch.Files) == 0 {
		return nil
	}

	part := &Part{
		ID:          util.Ascending("part"),
		MessageID:   runtimeConfig.Assistant.ID,
		SessionID:   r.GetSessionID(),
		Type:        types.PartTypePatch,
		Files:       patch.Files,
		Description: description,
	}

	row := &models.MessageCodeSnapshot{
		ID:          util.Ascending("csnap"),
		SessionID:   r.GetSessionID(),
		MessageID:   runtimeConfig.Assistant.ID,
		PartID:      part.ID,
		StartHash:   runtimeConfig.Assistant.Snapshot,
		EndHash:     finishSnapshot,
		Description: description,
		Files:       models.JSONField{Data: patch.Files},
		Created:     runtimeConfig.Assistant.Time.Created,
	}

	if err := r.db.Transaction(func(tx *gorm.DB) error {
		if _, err := storage.UpdatePart(tx, part); err != nil {
			return err
		}
		return storage.CreateMessageCodeSnapshot(tx, row)
	}); err != nil {
		return fmt.Errorf("persist code snapshot: %w", err)
	}

	if _, err := r.emitter.UpdatePart(part); err != nil {
		return fmt.Errorf("emit finish snapshot patch part: %w", err)
	}
	return nil
}
