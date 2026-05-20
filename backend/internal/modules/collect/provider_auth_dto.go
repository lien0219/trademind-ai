package collect

// Provider1688AuthStatusDTO mirrors collector GET /v1/providers/1688/auth-status.
type Provider1688AuthStatusDTO struct {
	Provider         string `json:"provider"`
	Status           string `json:"status"`
	LoggedIn         bool   `json:"loggedIn"`
	NeedVerification bool   `json:"needVerification"`
	Message          string `json:"message"`
	LastCheckedAt    string `json:"lastCheckedAt"`
	ProfilePath      string `json:"profilePath,omitempty"`
}

// Provider1688OpenLoginResultDTO mirrors collector POST /v1/providers/1688/open-login-browser.
type Provider1688OpenLoginResultDTO struct {
	ProfilePath string `json:"profilePath"`
	Message     string `json:"message"`
	AlreadyOpen bool   `json:"alreadyOpen"`
}
