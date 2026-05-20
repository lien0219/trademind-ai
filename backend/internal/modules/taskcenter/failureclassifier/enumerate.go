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
