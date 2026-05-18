package productcheck

// BlockedError is returned when publishing must stop due to readiness errors.
type BlockedError struct {
	Result *CheckProductReadinessResult
}

func (e *BlockedError) Error() string {
	return "product readiness check failed"
}

func (e *BlockedError) Unwrap() error { return nil }
