package session

import (
	"context"

	"matrixops-agent/types"
	database "pkgs/db"

	"gorm.io/gorm"
)

type SessionGraduatedCompactionOptions struct {
	DB             *gorm.DB
	SessionID      string
	Entries        []*types.MemoryEntry
	LimitTokens    int
	Force          bool
	UserPrompt     string
	OnSummaryDelta func(string)
}

func RunSessionGraduatedCompaction(options SessionGraduatedCompactionOptions) (GraduatedMemoryCompactionResult, error) {
	if options.DB == nil {
		return GraduatedMemoryCompactionResult{}, errDatabaseNotConfigured()
	}
	currentTokens := totalMemoryTokens(options.Entries)
	if options.LimitTokens <= 0 {
		options.LimitTokens = currentTokens
	}
	if options.LimitTokens <= 0 {
		return GraduatedMemoryCompactionResult{Skipped: true, SkipReason: "missing_limit"}, nil
	}

	ctx := context.Background()
	return RunGraduatedMemoryCompaction(GraduatedMemoryCompactionInput{
		SessionID:     options.SessionID,
		Entries:       options.Entries,
		LimitTokens:   options.LimitTokens,
		CurrentTokens: currentTokens,
		Config: GraduatedMemoryCompactionConfig{
			TriggerThresholdPercent: database.GetMemoryCompactionTriggerThresholdPercent(options.DB),
			TargetPercent:           database.GetMemoryCompactionTargetPercent(options.DB),
			L2ScopePercent:          database.GetMemoryCompactionL2ScopePercent(options.DB),
			Force:                   options.Force,
		},
		UserPrompt:     options.UserPrompt,
		OnSummaryDelta: options.OnSummaryDelta,
		Summarize: func(scopeMemory *types.Memory, userPrompt string, onDelta func(string)) (string, compactionPromptInfo, error) {
			summary, promptInfo, _, err := RunMemoryCompactionWorkerChat(MemoryCompactionWorkerChatOptions{
				Context:        ctx,
				DB:             options.DB,
				Memory:         scopeMemory,
				UserInput:      userPrompt,
				OnSummaryDelta: onDelta,
			})
			return summary, promptInfo, err
		},
	})
}

func errDatabaseNotConfigured() error {
	return &compactionConfigError{message: "database is not configured"}
}

type compactionConfigError struct {
	message string
}

func (e *compactionConfigError) Error() string {
	return e.message
}
