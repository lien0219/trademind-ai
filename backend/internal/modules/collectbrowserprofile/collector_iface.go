package collectbrowserprofile

import "context"

// CollectorGateway calls Node collector browser-profile APIs.
type CollectorGateway interface {
	OpenProfileLogin(ctx context.Context, profileKey, rawURL string) (message string, err error)
	CheckProfileAccess(ctx context.Context, profileKey, rawURL string) (*CheckResultDTO, error)
}
