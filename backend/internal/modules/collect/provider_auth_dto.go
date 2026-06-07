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
	ProfilePath string `json:"profilePath,omitempty"`
	Message     string `json:"message"`
	AlreadyOpen bool   `json:"alreadyOpen"`
}

// PinduoduoAuthEvidenceDTO safe summary from collector (no cookies/HTML).
type PinduoduoAuthEvidenceDTO struct {
	HasProductTitle bool `json:"hasProductTitle"`
	HasPrice        bool `json:"hasPrice"`
	HasMainImage    bool `json:"hasMainImage"`
	HasLoginText    bool `json:"hasLoginText"`
	HasWechatAuth   bool `json:"hasWechatAuth"`
	HasAppRedirect  bool `json:"hasAppRedirect"`
}

// ProviderPinduoduoAuthStatusDTO mirrors collector pinduoduo auth check responses.
type ProviderPinduoduoAuthStatusDTO struct {
	Provider         string                   `json:"provider"`
	ProfileKey       string                   `json:"profileKey"`
	Status           string                   `json:"status"`
	LoginStatus      string                   `json:"loginStatus"`
	LoggedIn         bool                     `json:"loggedIn"`
	NeedVerification bool                     `json:"needVerification"`
	Message          string                   `json:"message"`
	LastCheckedAt    string                   `json:"lastCheckedAt"`
	CheckedURL       string                   `json:"checkedUrl"`
	FinalURL         string                   `json:"finalUrl"`
	AccessStatus     string                   `json:"accessStatus"`
	URLType          string                   `json:"urlType"`
	CheckMode        string                   `json:"checkMode,omitempty"`
	Evidence         PinduoduoAuthEvidenceDTO `json:"evidence"`
	ProfilePath      string                   `json:"profilePath,omitempty"`
}

// ProviderPinduoduoOpenLoginResultDTO mirrors collector POST /v1/providers/pinduoduo/open-login-browser.
type ProviderPinduoduoOpenLoginResultDTO struct {
	ProfilePath string `json:"profilePath,omitempty"`
	Message     string `json:"message"`
	AlreadyOpen bool   `json:"alreadyOpen"`
}

// PinduoduoOpenLoginBody optional login entry URL.
type PinduoduoOpenLoginBody struct {
	URL string `json:"url"`
}

// PinduoduoCheckLoginBody optional URLs for login status check.
type PinduoduoCheckLoginBody struct {
	URL     string `json:"url"`
	TestURL string `json:"testUrl"`
}

// ProviderTaobaoTmallAuthStatusDTO mirrors collector taobao_tmall auth check responses.
type ProviderTaobaoTmallAuthStatusDTO struct {
	Provider         string `json:"provider"`
	ProfileKey       string `json:"profileKey"`
	Status           string `json:"status"`
	LoginStatus      string `json:"loginStatus"`
	LoggedIn         bool   `json:"loggedIn"`
	NeedVerification bool   `json:"needVerification"`
	Message          string `json:"message"`
	LastCheckedAt    string `json:"lastCheckedAt"`
	CheckedURL       string `json:"checkedUrl"`
	FinalURL         string `json:"finalUrl"`
	ProfilePath      string `json:"profilePath,omitempty"`
}

// ProviderTaobaoTmallOpenLoginResultDTO mirrors collector POST /v1/providers/taobao_tmall/open-login-browser.
type ProviderTaobaoTmallOpenLoginResultDTO struct {
	ProfilePath string `json:"profilePath,omitempty"`
	Message     string `json:"message"`
	AlreadyOpen bool   `json:"alreadyOpen"`
}

// TaobaoTmallOpenLoginBody optional login entry URL.
type TaobaoTmallOpenLoginBody struct {
	URL string `json:"url"`
}

// TaobaoTmallCheckLoginBody optional URLs for login status check.
type TaobaoTmallCheckLoginBody struct {
	URL     string `json:"url"`
	TestURL string `json:"testUrl"`
}
