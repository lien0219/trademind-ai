package collectruleai

import (
	"context"
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
)

type pageAnalyzerAdapter struct {
	client *collect.CollectorClient
}

func (a pageAnalyzerAdapter) AnalyzePage(ctx context.Context, rawURL string, opts map[string]any) (*PageStructureDigest, error) {
	if a.client == nil {
		return nil, fmt.Errorf("collector client unavailable")
	}
	d, err := a.client.AnalyzePage(ctx, rawURL, opts)
	if err != nil {
		return nil, err
	}
	return digestFromCollect(d), nil
}

type providerResolverAdapter struct {
	svc *collect.Service
}

func (a providerResolverAdapter) ResolveCollectProviders(ctx context.Context) []collect.CollectProviderDTO {
	if a.svc == nil {
		return nil
	}
	return a.svc.ResolveCollectProviders(ctx)
}

// NewPageAnalyzer wraps CollectorClient for collectruleai.Service.
func NewPageAnalyzer(client *collect.CollectorClient) PageAnalyzer {
	return pageAnalyzerAdapter{client: client}
}

// NewProviderResolver wraps collect.Service for platform checks.
func NewProviderResolver(svc *collect.Service) ProviderResolver {
	return providerResolverAdapter{svc: svc}
}

// DigestFromCollect converts collector digest to module type.
func DigestFromCollect(d *collect.AnalyzePageDigest) *PageStructureDigest {
	return digestFromCollect(d)
}
