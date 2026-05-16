package collectrule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
)

func clampRulePage(page, ps int) (int, int) {
	if page < 1 {
		page = 1
	}
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}
	return page, ps
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

// CollectorRunner invokes Node Collector HTTP API (implemented via collect.CollectorClient adapter).
type CollectorRunner interface {
	RunCollect(ctx context.Context, source, rawURL string, options map[string]any) (json.RawMessage, error)
}

// TaskRulePayload is persisted on collect_tasks.request_options and sent to Collector as options.* .
type TaskRulePayload struct {
	RuleID       string          `json:"ruleId"`
	RuleName     string          `json:"ruleName"`
	Domain       string          `json:"domain"`
	MatchPattern string          `json:"matchPattern,omitempty"`
	Rule         json.RawMessage `json:"rule"`
}

// Service manages collect_rules CRUD and resolves rules for custom tasks.
type Service struct {
	DB          *gorm.DB
	OpLog       *operationlog.Service
	Runner      CollectorRunner
	TestTimeout time.Duration
}

func (s *Service) hostnameFromURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme")
	}
	h := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if h == "" {
		return "", fmt.Errorf("missing host")
	}
	return h, nil
}

func domainMatches(host, domain string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	d := strings.ToLower(strings.TrimSpace(domain))
	if d == "" || h == "" {
		return false
	}
	return h == d || strings.HasSuffix(h, "."+d)
}

func (s *Service) ruleMatchesURL(rule *CollectRule, rawURL string) bool {
	if rule == nil {
		return false
	}
	host, err := s.hostnameFromURL(rawURL)
	if err != nil {
		return false
	}
	if !domainMatches(host, rule.Domain) {
		return false
	}
	mp := strings.TrimSpace(rule.MatchPattern)
	if mp == "" {
		return true
	}
	re, err := regexp.Compile(mp)
	if err != nil {
		return false
	}
	return re.MatchString(strings.TrimSpace(rawURL))
}

// ResolveEnabledRuleForCustom resolves one enabled rule by explicit id or URL auto-match.
func (s *Service) ResolveEnabledRuleForCustom(ctx context.Context, rawURL string, explicitRuleID *uuid.UUID) (*CollectRule, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("collectrule: no db")
	}
	urlStr := strings.TrimSpace(rawURL)
	if explicitRuleID != nil && *explicitRuleID != uuid.Nil {
		var rule CollectRule
		if err := s.DB.WithContext(ctx).
			Where("id = ? AND status = ?", *explicitRuleID, StatusEnabled).
			First(&rule).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("rule not found or disabled")
			}
			return nil, err
		}
		if !strings.EqualFold(rule.Source, SourceCustom) {
			return nil, fmt.Errorf("rule source mismatch")
		}
		if !s.ruleMatchesURL(&rule, urlStr) {
			return nil, fmt.Errorf("url does not match rule domain or pattern")
		}
		return &rule, nil
	}

	host, err := s.hostnameFromURL(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid url")
	}

	var rules []CollectRule
	if err := s.DB.WithContext(ctx).
		Where("status = ? AND source = ?", StatusEnabled, SourceCustom).
		Order("priority ASC, updated_at DESC").
		Find(&rules).Error; err != nil {
		return nil, err
	}
	for i := range rules {
		if domainMatches(host, rules[i].Domain) && s.ruleMatchesURL(&rules[i], urlStr) {
			return &rules[i], nil
		}
	}
	return nil, fmt.Errorf("no enabled rule matches this url domain")
}

// BuildTaskPayload serializes snapshot for collect_tasks.request_options.
func (s *Service) BuildTaskPayload(rule *CollectRule) ([]byte, error) {
	if rule == nil {
		return nil, fmt.Errorf("nil rule")
	}
	p := TaskRulePayload{
		RuleID:       rule.ID.String(),
		RuleName:     rule.Name,
		Domain:       rule.Domain,
		MatchPattern: rule.MatchPattern,
	}
	if len(rule.Rule) > 0 {
		p.Rule = json.RawMessage(rule.Rule)
	}
	return json.Marshal(p)
}

func (s *Service) List(ctx context.Context, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("collectrule: no db")
	}
	page, ps := clampRulePage(q.Page, q.PageSize)
	tx := s.DB.WithContext(ctx).Model(&CollectRule{})
	if v := strings.TrimSpace(q.Name); v != "" {
		tx = tx.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(v)+"%")
	}
	if v := strings.TrimSpace(q.Domain); v != "" {
		tx = tx.Where("LOWER(domain) LIKE ?", "%"+strings.ToLower(v)+"%")
	}
	if v := strings.TrimSpace(q.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	offset := (page - 1) * ps
	var rows []CollectRule
	if err := tx.Order("priority ASC, updated_at DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]RuleListItemDTO, 0, len(rows))
	for i := range rows {
		items = append(items, ruleToListDTO(&rows[i]))
	}
	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	if pages == 0 && total > 0 {
		pages = 1
	}
	return &ListResult{Items: items, Total: total, Page: page, PageSize: ps, TotalPages: pages}, nil
}

func (s *Service) GetDetail(ctx context.Context, id uuid.UUID) (*RuleDetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("collectrule: no db")
	}
	var r CollectRule
	if err := s.DB.WithContext(ctx).First(&r, "id = ?", id).Error; err != nil {
		return nil, err
	}
	d := ruleToDetailDTO(&r)
	return &d, nil
}

func normalizeNewRule(body CreateRuleBody, adminID *uuid.UUID) (*CollectRule, error) {
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	domain := strings.TrimSpace(strings.ToLower(body.Domain))
	if err := ValidateDomain(domain); err != nil {
		return nil, err
	}
	if err := ValidateMatchPattern(body.MatchPattern); err != nil {
		return nil, err
	}
	st := strings.TrimSpace(strings.ToLower(body.Status))
	if st == "" {
		st = StatusEnabled
	}
	if st != StatusEnabled && st != StatusDisabled {
		return nil, fmt.Errorf("invalid status")
	}
	if len(body.Rule) == 0 {
		return nil, fmt.Errorf("rule is required")
	}
	if err := ValidateRuleJSON(body.Rule); err != nil {
		return nil, err
	}
	pr := 100
	if body.Priority != nil {
		pr = clampPriority(*body.Priority)
	}
	return &CollectRule{
		Name:         name,
		Source:       SourceCustom,
		Domain:       domain,
		MatchPattern: strings.TrimSpace(body.MatchPattern),
		Status:       st,
		Priority:     pr,
		Rule:         datatypes.JSON(body.Rule),
		Remark:       strings.TrimSpace(body.Remark),
		CreatedBy:    adminID,
	}, nil
}

func (s *Service) Create(c *gin.Context, body CreateRuleBody, adminID *uuid.UUID) (*RuleDetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("collectrule: no db")
	}
	row, err := normalizeNewRule(body, adminID)
	if err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(row).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "collect.rule.create",
			Resource:   "collect_rule",
			ResourceID: row.ID.String(),
			Status:     "success",
			Message:    truncateRunes(fmt.Sprintf("name=%s domain=%s", row.Name, row.Domain), 2000),
		})
	}
	d := ruleToDetailDTO(row)
	return &d, nil
}

func (s *Service) Update(c *gin.Context, id uuid.UUID, body UpdateRuleBody, adminID *uuid.UUID) (*RuleDetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("collectrule: no db")
	}
	var row CollectRule
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if body.Name != nil {
		n := strings.TrimSpace(*body.Name)
		if n == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		updates["name"] = n
	}
	if body.Domain != nil {
		d := strings.TrimSpace(strings.ToLower(*body.Domain))
		if err := ValidateDomain(d); err != nil {
			return nil, err
		}
		updates["domain"] = d
	}
	if body.MatchPattern != nil {
		if err := ValidateMatchPattern(*body.MatchPattern); err != nil {
			return nil, err
		}
		updates["match_pattern"] = strings.TrimSpace(*body.MatchPattern)
	}
	if body.Priority != nil {
		updates["priority"] = clampPriority(*body.Priority)
	}
	if body.Status != nil {
		st := strings.TrimSpace(strings.ToLower(*body.Status))
		if st != StatusEnabled && st != StatusDisabled {
			return nil, fmt.Errorf("invalid status")
		}
		updates["status"] = st
	}
	if body.Remark != nil {
		updates["remark"] = strings.TrimSpace(*body.Remark)
	}
	if body.Rule != nil {
		if len(*body.Rule) == 0 {
			return nil, fmt.Errorf("rule cannot be empty")
		}
		if err := ValidateRuleJSON(*body.Rule); err != nil {
			return nil, err
		}
		updates["rule"] = datatypes.JSON(append(json.RawMessage(nil), *body.Rule...))
	}
	if len(updates) == 0 {
		d := ruleToDetailDTO(&row)
		return &d, nil
	}
	if err := s.DB.WithContext(c.Request.Context()).Model(&CollectRule{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "collect.rule.update",
			Resource:    "collect_rule",
			ResourceID:  id.String(),
			Status:      "success",
			Message:     truncateRunes(fmt.Sprintf("updated keys=%v", keysOf(updates)), 2000),
		})
	}
	d := ruleToDetailDTO(&row)
	return &d, nil
}

func keysOf(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func (s *Service) Delete(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("collectrule: no db")
	}
	res := s.DB.WithContext(c.Request.Context()).Delete(&CollectRule{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "collect.rule.delete",
			Resource:    "collect_rule",
			ResourceID:  id.String(),
			Status:      "success",
			Message:     "soft deleted",
		})
	}
	return nil
}

func (s *Service) SetStatus(c *gin.Context, id uuid.UUID, status string, adminID *uuid.UUID) (*RuleDetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("collectrule: no db")
	}
	st := strings.TrimSpace(strings.ToLower(status))
	if st != StatusEnabled && st != StatusDisabled {
		return nil, fmt.Errorf("invalid status")
	}
	var row CollectRule
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if row.Status == st {
		d := ruleToDetailDTO(&row)
		return &d, nil
	}
	if err := s.DB.WithContext(c.Request.Context()).Model(&CollectRule{}).
		Where("id = ?", id).
		Update("status", st).Error; err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	action := "collect.rule.enable"
	if st == StatusDisabled {
		action = "collect.rule.disable"
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      action,
			Resource:    "collect_rule",
			ResourceID:  id.String(),
			Status:      "success",
			Message:     fmt.Sprintf("status=%s", st),
		})
	}
	d := ruleToDetailDTO(&row)
	return &d, nil
}

// TestPreview runs Collector once without persisting tasks or products.
func (s *Service) TestPreview(c *gin.Context, id uuid.UUID, body TestRuleBody, adminID *uuid.UUID) (json.RawMessage, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("collectrule: no db")
	}
	if s.Runner == nil {
		return nil, fmt.Errorf("collector runner unavailable")
	}
	rawURL := strings.TrimSpace(body.URL)
	if err := validateHTTPURL(rawURL); err != nil {
		return nil, err
	}
	var rule CollectRule
	if err := s.DB.WithContext(c.Request.Context()).First(&rule, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if !strings.EqualFold(rule.Source, SourceCustom) {
		return nil, fmt.Errorf("unsupported rule source")
	}
	if !s.ruleMatchesURL(&rule, rawURL) {
		return nil, fmt.Errorf("url does not match rule domain or pattern")
	}

	timeout := s.TestTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()

	opts := map[string]any{
		"ruleId":       rule.ID.String(),
		"ruleName":     rule.Name,
		"domain":       rule.Domain,
		"matchPattern": rule.MatchPattern,
		"rule":         json.RawMessage(rule.Rule),
	}

	out, err := s.Runner.RunCollect(ctx, "custom", rawURL, opts)
	if err != nil {
		if s.OpLog != nil {
			msg := truncateRunes(err.Error(), 1500)
			_ = s.OpLog.Write(c, operationlog.WriteOpts{
				AdminUserID: adminID,
				Action:      "collect.rule.test.failed",
				Resource:    "collect_rule",
				ResourceID:  id.String(),
				Status:      "failed",
				Message:     msg,
			})
		}
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "collect.rule.test.success",
			Resource:    "collect_rule",
			ResourceID:  id.String(),
			Status:      "success",
			Message:     truncateRunes(fmt.Sprintf("url_len=%d", len(rawURL)), 2000),
		})
	}
	return out, nil
}
