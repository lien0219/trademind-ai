package douyinshop

import (
	"context"
	"strings"
)

// LoadRuntimeFromBridge reads runtime state from global settings via ShopsBridge.
func LoadRuntimeFromBridge(ctx context.Context) (RuntimeState, StaleTimeouts, RuntimeConfig, error) {
	var cfg RuntimeConfig
	defStale := DefaultStaleTimeouts()
	if bridges == nil {
		return RuntimeState{Status: RuntimeNormal}, defStale, cfg, nil
	}
	m, err := bridges.DouyinGlobalSettings(ctx)
	if err != nil {
		return RuntimeState{}, defStale, cfg, err
	}
	rt, stale := RuntimeStateFromMergedMap(m)
	cfg, err = RuntimeFromMergedMap(m)
	if err != nil {
		return rt, stale, cfg, err
	}
	return rt, stale, cfg, nil
}

// GuardWorker checks worker execution for a Douyin feature.
func GuardWorker(ctx context.Context, feature string, isWrite bool) *Error {
	rt, _, cfg, err := LoadRuntimeFromBridge(ctx)
	if err != nil {
		return NewError(CodeDouyinFeatureDisabled, RuntimeBlockedUserMessage, "", err.Error(), "")
	}
	return CheckWorkerExecution(WorkerGuardInput{
		Config:  cfg,
		Runtime: rt,
		Feature: strings.TrimSpace(feature),
		IsWrite: isWrite,
	})
}
