package failureclassifier

// AllCategories lists supported failure_category values (API enums).
func AllCategories() []string {
	return []string{
		CategoryPlatformAuth,
		CategoryPlatformPermission,
		CategoryPlatformRateLimit,
		CategoryPlatformAPIError,
		CategoryPlatformConfigIncomplete,
		CategoryNetworkTimeout,
		CategoryCollectorBlocked,
		CategoryCollectorPlatformLogin,
		CategoryCollectorMissingImages,
		CategoryCollectorMissingPrice,
		CategoryCollectorEvaluateScript,
		CategoryCollectorInvalidURL,
		CategoryAIProviderError,
		CategoryAIConfigIncomplete,
		CategoryImageProviderError,
		CategoryStorageError,
		CategoryValidationError,
		CategoryInventoryMappingMissing,
		CategorySKUMappingMissing,
		CategoryWorkerLeaseExpired,
		CategorySystemError,
		CategoryUnknown,
		// AI product text batch review (aiproducttext module)
		"ai_text_generation_failed",
		"ai_text_apply_conflict",
		"ai_text_apply_failed",
		"ai_text_undo_failed",
		"ai_text_quality_warning",
	}
}

// AllSeverities lists severity labels.
func AllSeverities() []string {
	return []string{
		SeverityLow,
		SeverityMedium,
		SeverityHigh,
		SeverityCritical,
	}
}
