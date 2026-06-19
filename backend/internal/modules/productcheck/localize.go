package productcheck

import (
	"github.com/trademind-ai/trademind/backend/internal/pkg/opslabels"
)

// LocalizeCheckItem enriches a check item with user-facing Chinese fields.
func LocalizeCheckItem(c CheckItem) CheckItem {
	loc := opslabels.LocalizeReadinessIssue(c.Code, c.Level, c.Message, c.Suggestion, c.Group, c.RelatedResourceType, c.RelatedResourceID)
	out := c
	out.Title = loc.Title
	if loc.Message != "" {
		out.Message = loc.Message
	}
	if loc.Suggestion != "" {
		out.Suggestion = loc.Suggestion
	}
	if loc.Severity != "" {
		out.Level = loc.Severity
	}
	if len(loc.TechnicalDetails) > 0 {
		out.TechnicalDetails = loc.TechnicalDetails
	}
	return out
}

// LocalizeReadinessResult adds status labels and localized checks.
func LocalizeReadinessResult(res *CheckProductReadinessResult) *CheckProductReadinessResult {
	if res == nil {
		return nil
	}
	out := *res
	out.StatusLabel = opslabels.StatusLabel(res.Status)
	out.ResultLabel = opslabels.StatusLabel(res.Result)
	if res.Result == "passed" {
		out.ResultLabel = "检查通过"
	}
	checks := make([]CheckItem, len(res.Checks))
	for i, c := range res.Checks {
		checks[i] = LocalizeCheckItem(c)
	}
	out.Checks = checks
	return &out
}
