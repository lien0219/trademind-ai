package demoseed

import "errors"

// ErrProductionForbidden is returned when demo seed is attempted in production.
var ErrProductionForbidden = errors.New("demoseed: forbidden in production environment")
