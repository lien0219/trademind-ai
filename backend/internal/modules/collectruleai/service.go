package collectruleai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/aiprompt"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectrule"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/aimodelparse"
	"github.com/trademind-ai/trademind/backend/internal/pkg/collectdomain"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
)

const (
	defaultDigestMaxBytes  = 20_000
	defaultMaxCandidates   = 20
	defaultTargetFieldsCSV = "title,price,mainImages,descriptionImages,attributes"
)

type PageStructureDigest struct {
	URL          string `json:"url"`
	FinalURL     string `json:"finalUrl"`
	AccessStatus string `json:"accessStatus"`
	Title        string `json:"title"`
	Meta         struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		OgTitle     string `json:"ogTitle"`
		OgImage     string `json:"ogImage"`
	} `json:"meta"`
	Candidates json.RawMessage `json:"candidates"`
	DomHints   []string        `json:"domHints"`
}

func digestFromCollect(d *collect.AnalyzePageDigest) *PageStructureDigest {
	if d == nil {
		return nil
	}
	out := &PageStructureDigest{
		URL:          d.URL,
		FinalURL:     d.FinalURL,
		AccessStatus: d.AccessStatus,
		Title:        d.Title,
		Candidates:   d.Candidates,
		DomHints:     d.DomHints,
	}
	if len(d.Meta) > 0 {
		_ = json.Unmarshal(d.Meta, &out.Meta)
	}
	return out
}

type PageAnalyzer interface {
	AnalyzePage(ctx context.Context, rawURL string, opts map[string]any) (*PageStructureDigest, error)
}

type ProviderResolver interface {
	ResolveCollectProviders(ctx context.Context) []collect.CollectProviderDTO
}

type ProfileEnricher interface {
	EnrichCollectorOptions(ctx context.Context, opts map[string]any, profileID *uuid.UUID, useBrowserProfile bool, rawURL string) error
}

type RuleCreator interface {
	CreateFromAI(c *gin.Context, body collectrule.CreateRuleBody, adminID *uuid.UUID) (*collectrule.RuleDetailDTO, error)
}

// Service generates custom collect rules via AI from page digests.
type Service struct {
	Settings    *settings.Service
	Prompts     *aiprompt.Service
	AIGateway   *aigate.Gateway
	Analyzer    PageAnalyzer
	Runner      collectrule.CollectorRunner
	Profiles    ProfileEnricher
	Providers   ProviderResolver
	Rules       RuleCreator
	OpLog       *operationlog.Service
	TestTimeout time.Duration
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}

func truncateURLForLog(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return truncateRunes(raw, 80)
	}
	host := strings.ToLower(u.Hostname())
	path := u.Path
	if len(path) > 40 {
		path = path[:40] + "…"
	}
	return host + path
}

func (s *Service) readConfig(ctx context.Context) (enabled bool, digestMax int, maxCand int, defaultFields []string) {
	enabled = true
	digestMax = defaultDigestMaxBytes
	maxCand = defaultMaxCandidates
	defaultFields = []string{"title", "price", "mainImages", "descriptionImages", "attributes"}
	if s == nil || s.Settings == nil {
		return
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "collector")
	if err != nil {
		return
	}
	v := strings.TrimSpace(strings.ToLower(m["collect_rule_ai_enabled"]))
	if v == "0" || v == "false" || v == "off" {
		enabled = false
	}
	if n := parseIntSetting(m["collect_rule_ai_max_html_digest_size"], digestMax); n > 0 {
		digestMax = n
	}
	if n := parseIntSetting(m["collect_rule_ai_max_candidates"], maxCand); n > 0 {
		maxCand = n
	}
	if csv := strings.TrimSpace(m["collect_rule_ai_default_target_fields"]); csv != "" {
		parts := strings.Split(csv, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			defaultFields = out
		}
	}
	return
}

func parseIntSetting(v string, def int) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return def
	}
	return n
}

func (s *Service) isAIConfigured(ctx context.Context) bool {
	if s == nil || s.Settings == nil {
		return false
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "ai")
	if err != nil {
		return false
	}
	provider := strings.TrimSpace(m["provider"])
	model := strings.TrimSpace(m["model"])
	key := strings.TrimSpace(m["api_key"])
	return provider != "" && model != "" && key != ""
}

func validateHTTPURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("url must start with http:// or https://")
	}
	if strings.TrimSpace(u.Hostname()) == "" {
		return fmt.Errorf("url host required")
	}
	return nil
}

func normalizeTargetFields(in []string, defaults []string) []string {
	allowed := map[string]struct{}{
		"title": {}, "price": {}, "mainImages": {}, "descriptionImages": {}, "attributes": {}, "skus": {},
	}
	if len(in) == 0 {
		return append([]string(nil), defaults...)
	}
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, f := range in {
		f = strings.TrimSpace(f)
		if _, ok := allowed[f]; !ok {
			continue
		}
		if _, dup := seen[f]; dup {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	if len(out) == 0 {
		return append([]string(nil), defaults...)
	}
	return out
}

func resolveDomain(bodyDomain, urlStr string) string {
	d := collectrule.NormalizeRuleDomain(bodyDomain)
	if d != "" {
		return d
	}
	host := collectdomain.HostnameFromURL(urlStr)
	if host == "" {
		return ""
	}
	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return host
}

func suggestedRuleName(domain, pageTitle string) string {
	d := strings.TrimSpace(domain)
	if d == "" {
		d = "custom"
	}
	title := truncateRunes(strings.TrimSpace(pageTitle), 24)
	if title != "" {
		return fmt.Sprintf("%s-%s", d, title)
	}
	return d + "-自定义"
}

func trimDigestForAI(digest *PageStructureDigest, maxBytes int) ([]byte, error) {
	if digest == nil {
		return nil, fmt.Errorf("empty digest")
	}
	raw, err := json.Marshal(digest)
	if err != nil {
		return nil, err
	}
	if len(raw) <= maxBytes {
		return raw, nil
	}
	// Drop textSamples / imageSamples by re-marshaling a slim copy
	slim := map[string]any{
		"url":          digest.URL,
		"finalUrl":     digest.FinalURL,
		"accessStatus": digest.AccessStatus,
		"title":        truncateRunes(digest.Title, 200),
		"meta":         digest.Meta,
		"candidates":   json.RawMessage(digest.Candidates),
		"domHints":     digest.DomHints,
	}
	raw, err = json.Marshal(slim)
	if err != nil {
		return nil, err
	}
	if len(raw) > maxBytes {
		return raw[:maxBytes], nil
	}
	return raw, nil
}

func parseAIRuleResponse(content string) (aiRuleOutput, error) {
	var out aiRuleOutput
	norm := aimodelparse.NormalizeJSONContent(content)
	if err := json.Unmarshal([]byte(norm), &out); err != nil {
		return out, fmt.Errorf("parse ai json: %w", err)
	}
	if len(out.Rule) == 0 {
		return out, fmt.Errorf("AI response missing rule")
	}
	return out, nil
}

// GenerateCollectRuleWithAI analyzes page, calls AI, validates rule, and runs rule test.
func (s *Service) GenerateCollectRuleWithAI(c *gin.Context, body GenerateBody, adminID *uuid.UUID) (*GenerateResultDTO, error) {
	if s == nil {
		return nil, fmt.Errorf("collect rule ai unavailable")
	}
	ctx := c.Request.Context()
	rawURL := strings.TrimSpace(body.URL)
	if err := validateHTTPURL(rawURL); err != nil {
		return nil, err
	}

	enabled, digestMax, maxCand, defaultFields := s.readConfig(ctx)
	if !enabled {
		return nil, fmt.Errorf("AI 生成采集规则已关闭，请在「采集设置 → 自定义链接」中开启")
	}
	if !s.isAIConfigured(ctx) {
		return nil, fmt.Errorf("请先到「设置 → AI 设置」配置并测试 AI 后，再使用 AI 生成采集规则")
	}
	if s.Analyzer == nil || s.Runner == nil || s.AIGateway == nil || s.Prompts == nil {
		return nil, fmt.Errorf("collect rule ai dependencies unavailable")
	}

	plannedHint, blockErr := checkPlatformForAIGenerate(ctx, s.Providers, rawURL)
	if blockErr != nil {
		return nil, blockErr
	}

	targetFields := normalizeTargetFields(body.TargetFields, defaultFields)
	domain := resolveDomain(body.Domain, rawURL)
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if err := collectrule.ValidateDomain(domain); err != nil {
		return nil, err
	}

	analyzeOpts := map[string]any{"maxCandidates": maxCand}
	if s.Profiles != nil && body.UseBrowserProfile && body.ProfileID != nil {
		pid, err := uuid.Parse(strings.TrimSpace(*body.ProfileID))
		if err != nil {
			return nil, fmt.Errorf("invalid profileId")
		}
		if err := s.Profiles.EnrichCollectorOptions(ctx, analyzeOpts, &pid, true, rawURL); err != nil {
			return nil, err
		}
	}

	timeout := s.TestTimeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	analyzeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	digest, err := s.Analyzer.AnalyzePage(analyzeCtx, rawURL, analyzeOpts)
	if err != nil {
		s.writeOpLog(c, adminID, "collect.rule.ai_generate_failed", rawURL, domain, body.RuleName, targetFields, "", 0, err.Error())
		return nil, err
	}
	if digest == nil {
		return nil, fmt.Errorf("page analyze returned empty")
	}

	switch strings.TrimSpace(digest.AccessStatus) {
	case "login_required":
		return nil, fmt.Errorf("页面需要登录，请启用浏览器 Profile 并完成登录后重试")
	case "verify_required", "blocked":
		return nil, fmt.Errorf("页面需要验证或已被风控拦截，无法自动生成规则")
	case "timeout", "navigation_failed":
		return nil, fmt.Errorf("无法打开页面：%s", digest.AccessStatus)
	}

	digestJSON, err := trimDigestForAI(digest, digestMax)
	if err != nil {
		return nil, err
	}

	promptRow, err := s.Prompts.GetEnabledByCode(ctx, aiprompt.CodeCollectRuleGenerate)
	if err != nil {
		return nil, fmt.Errorf("prompt collect_rule_generate not configured: %w", err)
	}

	vars := map[string]string{
		"url":          truncateURLForLog(rawURL),
		"domain":       domain,
		"targetFields": strings.Join(targetFields, ", "),
		"pageDigest":   string(digestJSON),
	}
	sys := aiprompt.ReplaceVariables(promptRow.SystemPrompt, vars)
	user := aiprompt.ReplaceVariables(promptRow.UserPrompt, vars)

	msgs := []aigate.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
	req := aigate.ChatRequest{
		Messages:       msgs,
		ResponseFormat: &aigate.ResponseFormat{Type: "json_object"},
	}
	if strings.TrimSpace(promptRow.Model) != "" {
		req.Model = strings.TrimSpace(promptRow.Model)
	}
	req.Temperature = promptRow.Temperature
	if promptRow.MaxTokens > 0 {
		req.MaxTokens = promptRow.MaxTokens
	}
	if req.MaxTokens < 2048 {
		req.MaxTokens = 2048
	}

	resp, err := s.AIGateway.Chat(ctx, req)
	if err != nil {
		s.writeOpLog(c, adminID, "collect.rule.ai_generate_failed", rawURL, domain, body.RuleName, targetFields, "AI_CHAT_FAILED", 0, err.Error())
		return nil, fmt.Errorf("AI 生成失败：%w", err)
	}

	aiOut, err := parseAIRuleResponse(resp.Content)
	if err != nil {
		s.writeOpLog(c, adminID, "collect.rule.ai_generate_failed", rawURL, domain, body.RuleName, targetFields, "AI_RULE_PARSE_FAILED", 0, err.Error())
		return nil, fmt.Errorf("%v: %w", ErrAIRuleInvalid, err)
	}

	normRule, err := normalizeAndValidateRule(aiOut.Rule)
	if err != nil {
		s.writeOpLog(c, adminID, "collect.rule.ai_generate_failed", rawURL, domain, body.RuleName, targetFields, "AI_RULE_INVALID", aiOut.Confidence, err.Error())
		return nil, err
	}

	testResult, testErr := s.runRuleTest(c, rawURL, domain, normRule, body, adminID)
	warnings := append([]string(nil), aiOut.Warnings...)
	if testErr != nil {
		warnings = append(warnings, "rule_test_failed:"+truncateRunes(testErr.Error(), 120))
	} else if testResult != nil {
		warnings = appendQualityTestWarnings(warnings, testResult)
		if testResult.QualityScore != nil {
			if score, ok := testResult.QualityScore["score"].(float64); ok && score < 50 {
				warnings = append(warnings, "规则测试质量偏低，请根据测试结果调整 selector 或重新生成。")
			}
		}
	}

	nameHint := strings.TrimSpace(body.RuleName)
	if nameHint == "" {
		nameHint = suggestedRuleName(domain, digest.Title)
	}

	s.writeOpLog(c, adminID, "collect.rule.ai_generate", rawURL, domain, nameHint, targetFields, "", aiOut.Confidence, "")

	return &GenerateResultDTO{
		Rule:          normRule,
		Domain:        domain,
		SuggestedName: nameHint,
		Confidence:    aiOut.Confidence,
		Explanation:   aiOut.Explanation,
		Warnings:      warnings,
		TestResult:    testResult,
		PlannedHint:   plannedHint,
	}, nil
}

func (s *Service) runRuleTest(
	c *gin.Context,
	rawURL, domain string,
	rule json.RawMessage,
	body GenerateBody,
	adminID *uuid.UUID,
) (*collectrule.RuleTestResultDTO, error) {
	timeout := s.TestTimeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()

	opts := map[string]any{
		"ruleId":       "ai-preview",
		"ruleName":     "AI Preview",
		"domain":       domain,
		"matchPattern": "",
		"rule":         json.RawMessage(rule),
	}
	if s.Profiles != nil && body.UseBrowserProfile && body.ProfileID != nil {
		pid, err := uuid.Parse(strings.TrimSpace(*body.ProfileID))
		if err == nil {
			_ = s.Profiles.EnrichCollectorOptions(ctx, opts, &pid, true, rawURL)
		}
	}
	out, err := s.Runner.CustomRuleTest(ctx, rawURL, opts)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) GenerateAndSave(c *gin.Context, body GenerateAndSaveBody, adminID *uuid.UUID) (*collectrule.RuleDetailDTO, error) {
	gen, err := s.GenerateCollectRuleWithAI(c, body.GenerateBody, adminID)
	if err != nil {
		return nil, err
	}
	if s.Rules == nil {
		return nil, fmt.Errorf("rule creator unavailable")
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = gen.SuggestedName
	}
	pr := 100
	if body.Priority != nil {
		pr = *body.Priority
	}
	createBody := collectrule.CreateRuleBody{
		Name:     name,
		Domain:   gen.Domain,
		Priority: &pr,
		Status:   collectrule.StatusEnabled,
		Rule:     gen.Rule,
	}
	out, err := s.Rules.CreateFromAI(c, createBody, adminID)
	if err != nil {
		s.writeOpLog(c, adminID, "collect.rule.ai_generate_failed", body.URL, gen.Domain, name, body.TargetFields, "SAVE_FAILED", gen.Confidence, err.Error())
		return nil, err
	}
	s.writeOpLog(c, adminID, "collect.rule.ai_save", body.URL, gen.Domain, name, body.TargetFields, "", gen.Confidence, out.ID.String())
	return out, nil
}

func (s *Service) writeOpLog(
	c *gin.Context,
	adminID *uuid.UUID,
	action, rawURL, domain, ruleName string,
	targetFields []string,
	errorCode string,
	confidence float64,
	errMsg string,
) {
	if s == nil || s.OpLog == nil {
		return
	}
	status := "success"
	if strings.Contains(action, "failed") || errMsg != "" {
		status = "failed"
	}
	msg := fmt.Sprintf(
		"domain=%s url=%s ruleName=%s fields=%s confidence=%.2f errorCode=%s",
		domain,
		truncateURLForLog(rawURL),
		truncateRunes(ruleName, 64),
		strings.Join(targetFields, ","),
		confidence,
		errorCode,
	)
	if errMsg != "" {
		msg += " err=" + truncateRunes(errMsg, 200)
	}
	_ = s.OpLog.Write(c, operationlog.WriteOpts{
		AdminUserID: adminID,
		Action:      action,
		Resource:    "collect_rule",
		Status:      status,
		Message:     truncateRunes(msg, 2000),
	})
}

// IsPlatformBlock reports whether err is dedicated-provider conflict.
func IsPlatformBlock(err error) (*platformBlockError, bool) {
	var pe *platformBlockError
	if errors.As(err, &pe) {
		return pe, true
	}
	return nil, false
}

func appendQualityTestWarnings(warnings []string, tr *collectrule.RuleTestResultDTO) []string {
	if tr == nil {
		return warnings
	}
	if ef := tr.ExtractedFields; ef != nil {
		if suspect, ok := ef["titleSuspectWrong"].(bool); ok && suspect {
			warnings = append(warnings, "当前标题可能不是商品标题，请调整商品标题对应的页面位置，或重新使用 AI 生成规则。")
		}
	}
	if qs := tr.QualityScore; qs != nil {
		if hints, ok := qs["hints"].([]interface{}); ok {
			for _, h := range hints {
				if s, ok := h.(string); ok && strings.TrimSpace(s) != "" {
					warnings = append(warnings, s)
				}
			}
		}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(warnings))
	for _, w := range warnings {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		if _, dup := seen[w]; dup {
			continue
		}
		seen[w] = struct{}{}
		out = append(out, w)
	}
	return out
}
