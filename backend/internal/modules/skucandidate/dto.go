package skucandidate

// CandidateSource is why a SKU appears in suggestions (for UI / audit summaries).
type CandidateSource string

const (
	SourceHistoryManualBind  CandidateSource = "history_manual_bind"
	SourcePublicationMapping CandidateSource = "publication_mapping"
	SourceLocalSKUCode       CandidateSource = "local_sku_code"
	SourceTitleSimilarity    CandidateSource = "title_similarity"
	SourceAttrsSimilarity    CandidateSource = "attrs_similarity"
)

const defaultMinConfidence = 40

// CandidateDTO is a read-only SKU hint for operators (never persisted by this module).
type CandidateDTO struct {
	ProductID       string          `json:"productId"`
	ProductTitle    string          `json:"productTitle"`
	ProductSKUID    string          `json:"productSkuId"`
	SKUCode         string          `json:"skuCode"`
	SKUName         string          `json:"skuName"`
	Stock           *int            `json:"stock,omitempty"`
	Attrs           map[string]any  `json:"attrs,omitempty"`
	Confidence      int             `json:"confidence"`
	Reason          string          `json:"reason"`
	MatchSignals    []string        `json:"matchSignals,omitempty"`
	Source          CandidateSource `json:"source"`
	SourceBreakdown map[string]int  `json:"sourceBreakdown,omitempty"`
}

// SuggestOpts controls suggestion generation (read-only).
type SuggestOpts struct {
	Limit                int  // default 10, max 20
	IncludeLowConfidence bool // if false (default), drop confidence < defaultMinConfidence
}

func (o SuggestOpts) normalized() suggestOptsResolved {
	out := suggestOptsResolved{Limit: o.Limit, IncludeLowConfidence: o.IncludeLowConfidence}
	if out.Limit <= 0 {
		out.Limit = 10
	}
	if out.Limit > 20 {
		out.Limit = 20
	}
	return out
}

type suggestOptsResolved struct {
	Limit                int
	IncludeLowConfidence bool
}

// ItemCandidatesDTO is GET /order-items/:id/sku-candidates payload.
type ItemCandidatesDTO struct {
	OrderItemID string         `json:"orderItemId"`
	List        []CandidateDTO `json:"list"`
}

// BatchRequest is POST /orders/:id/sku-candidates/batch body.
type BatchRequest struct {
	OrderItemIDs         []string `json:"orderItemIds"`
	Limit                int      `json:"limit"`
	IncludeLowConfidence *bool    `json:"includeLowConfidence"`
}

// BatchResponse wraps per-line results.
type BatchResponse struct {
	OrderID string              `json:"orderId"`
	Items   []ItemCandidatesDTO `json:"items"`
}
