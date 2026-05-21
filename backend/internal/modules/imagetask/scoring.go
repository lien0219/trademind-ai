package imagetask

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"time"

	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
)

// ImageScore is the structured scoring result for product images.
type ImageScore struct {
	OverallScore           float64  `json:"overallScore"`
	ClarityScore           float64  `json:"clarityScore"`
	CleanlinessScore       float64  `json:"cleanlinessScore"`
	CompositionScore       float64  `json:"compositionScore"`
	MainSuitabilityScore   float64  `json:"mainSuitabilityScore"`
	DetailSuitabilityScore float64  `json:"detailSuitabilityScore"`
	Issues                 []string `json:"issues"`
	Suggestion             string   `json:"suggestion"`
	Width                  int      `json:"width,omitempty"`
	Height                 int      `json:"height,omitempty"`
	Source                 string   `json:"source,omitempty"`
}

func scoreJSONFromScore(s ImageScore) ([]byte, error) {
	return json.Marshal(s)
}

func parseScoreJSON(raw []byte) (*ImageScore, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty score")
	}
	var s ImageScore
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func heuristicScore(width, height int, imageType string) ImageScore {
	clarity := 60.0
	if width >= 800 && height >= 800 {
		clarity = 75
	}
	if width >= 1200 && height >= 1200 {
		clarity = 85
	}
	clean := 70.0
	comp := 72.0
	mainSuit := 65.0
	detailSuit := 65.0
	if strings.EqualFold(imageType, "main") {
		mainSuit = 78
	}
	if strings.EqualFold(imageType, "detail") {
		detailSuit = 78
	}
	overall := (clarity + clean + comp + mainSuit + detailSuit) / 5
	return ImageScore{
		OverallScore:           round2(overall),
		ClarityScore:           round2(clarity),
		CleanlinessScore:       round2(clean),
		CompositionScore:       round2(comp),
		MainSuitabilityScore:   round2(mainSuit),
		DetailSuitabilityScore: round2(detailSuit),
		Issues:                 []string{},
		Suggestion:             "Heuristic score based on resolution; configure AI Provider for richer analysis.",
		Width:                  width,
		Height:                 height,
		Source:                 "heuristic",
	}
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func probeImageSize(ctx context.Context, imageURL string) (int, int, error) {
	u := strings.TrimSpace(imageURL)
	if u == "" {
		return 0, 0, fmt.Errorf("empty image url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, 0, err
	}
	cli := &http.Client{Timeout: 20 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, 0, fmt.Errorf("probe image HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return 0, 0, err
	}
	cfg, _, err := image.DecodeConfig(bytesReader(data))
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

type bytesReader []byte

func (b bytesReader) Read(p []byte) (int, error) {
	if len(b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, b)
	return n, nil
}

func (s *Service) scoreImageURL(ctx context.Context, imageURL, imageType, productTitle string) (ImageScore, error) {
	w, h, err := probeImageSize(ctx, imageURL)
	if err != nil {
		w, h = 0, 0
	}
	base := heuristicScore(w, h, imageType)
	if s == nil || s.AIGateway == nil {
		return base, nil
	}
	prompt := fmt.Sprintf(`You are an ecommerce product image quality analyst. Analyze the product image at URL: %s
Product title: %s
Image role: %s
Image size: %dx%d

Return ONLY valid JSON with keys:
overallScore, clarityScore, cleanlinessScore, compositionScore, mainSuitabilityScore, detailSuitabilityScore, issues (array of strings), suggestion (string)
Scores are 0-100 numbers.`, imageURL, strings.TrimSpace(productTitle), imageType, w, h)
	resp, err := s.AIGateway.Chat(ctx, aigate.ChatRequest{
		Messages: []aigate.Message{{Role: "user", Content: prompt}},
		ResponseFormat: &aigate.ResponseFormat{
			Type: "json_object",
		},
		MaxTokens: 800,
	})
	if err != nil {
		base.Suggestion = "AI scoring unavailable: " + truncateMsg(err.Error(), 120)
		return base, nil
	}
	content := strings.TrimSpace(resp.Content)
	var aiScore ImageScore
	if err := json.Unmarshal([]byte(content), &aiScore); err != nil {
		base.Suggestion = "AI scoring parse failed; using heuristic scores."
		return base, nil
	}
	if aiScore.OverallScore <= 0 {
		aiScore.OverallScore = base.OverallScore
	}
	aiScore.Width = w
	aiScore.Height = h
	aiScore.Source = "ai"
	return aiScore, nil
}

func weightedOverall(s ImageScore) float64 {
	return s.OverallScore*0.35 +
		s.MainSuitabilityScore*0.25 +
		s.ClarityScore*0.15 +
		s.CleanlinessScore*0.10 +
		s.CompositionScore*0.10 +
		s.DetailSuitabilityScore*0.05
}
