package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/providers/email"
	"github.com/trademind-ai/trademind/backend/internal/providers/email/smtp"
	"gorm.io/gorm"
)

type sendEmailCodeBody struct {
	Email string `json:"email" binding:"required,email"`
	Scene string `json:"scene" binding:"required,oneof=register"`
}

func (h *Handler) SendEmailCode(c *gin.Context) {
	if h.Redis == nil {
		response.Fail(c, 503, response.CodeInternalError, "redis unavailable")
		return
	}
	var body sendEmailCodeBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	emailAddr := strings.ToLower(strings.TrimSpace(body.Email))

	// Check if already registered
	if body.Scene == "register" {
		_, err := h.Admins.ByEmail(c.Request.Context(), emailAddr)
		if err == nil {
			response.Fail(c, 400, response.CodeBadRequest, "email already registered")
			return
		} else if err != gorm.ErrRecordNotFound {
			response.Fail(c, 500, response.CodeInternalError, "database error")
			return
		}
	}

	// Check rate limit: 60s cooldown
	cooldownKey := fmt.Sprintf("email_code_cooldown:%s:%s", body.Scene, emailAddr)
	exists, err := h.Redis.Exists(c.Request.Context(), cooldownKey).Result()
	if err != nil {
		response.Fail(c, 500, response.CodeInternalError, "redis error")
		return
	}
	if exists > 0 {
		response.Fail(c, 429, response.CodeBadRequest, "please wait before sending again")
		return
	}

	// Check rate limit: hourly limit
	hourlyKey := fmt.Sprintf("email_code_hourly:%s:%s", body.Scene, emailAddr)
	count, _ := h.Redis.Get(c.Request.Context(), hourlyKey).Int()
	if count >= 5 {
		response.Fail(c, 429, response.CodeBadRequest, "hourly limit reached")
		return
	}

	// Generate code
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	code := fmt.Sprintf("%06d", n.Int64())

	// Send email
	if err := h.sendCodeEmail(c.Request.Context(), emailAddr, code); err != nil {
		if h.OpLog != nil {
			_ = h.OpLog.Write(c, operationlog.WriteOpts{
				Username: emailAddr,
				Action:   "email_code.send",
				Resource: "auth",
				Status:   "failed",
				Message:  "failed to send email",
			})
		}
		response.Fail(c, 500, response.CodeInternalError, "failed to send email")
		return
	}

	// Save to Redis
	codeKey := fmt.Sprintf("email_code:%s:%s", body.Scene, emailAddr)
	h.Redis.Set(c.Request.Context(), codeKey, code, 10*time.Minute)
	h.Redis.Set(c.Request.Context(), cooldownKey, "1", 60*time.Second)
	
	if count == 0 {
		h.Redis.Set(c.Request.Context(), hourlyKey, 1, time.Hour)
	} else {
		h.Redis.Incr(c.Request.Context(), hourlyKey)
	}

	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			Username: emailAddr,
			Action:   "email_code.send",
			Resource: "auth",
			Status:   "success",
		})
	}

	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) sendCodeEmail(ctx context.Context, to, code string) error {
	m, err := h.Settings.PlainByGroup(ctx, 0, "email")
	if err != nil {
		return err
	}
	providerStr := strings.TrimSpace(m["provider"])
	if providerStr == "" || providerStr == "smtp" {
		port, _ := strconv.Atoi(m["smtp_port"])
		cfg := smtp.Config{
			Host:     m["smtp_host"],
			Port:     port,
			Username: m["smtp_username"],
			Password: m["smtp_password"],
			FromName: m["smtp_from_name"],
			From:     m["smtp_from"],
			UseTLS:   m["smtp_use_tls"] == "true",
			UseSSL:   m["smtp_use_ssl"] == "true",
		}
		if cfg.Host == "" || cfg.From == "" {
			return fmt.Errorf("email settings incomplete")
		}
		p := smtp.NewProvider(cfg)
		return p.Send(ctx, email.SendEmailRequest{
			To:          to,
			Subject:     "Your Verification Code - TradeMind",
			Content:     fmt.Sprintf("Your verification code is: %s. It will expire in 10 minutes.", code),
		})
	}
	return fmt.Errorf("unsupported email provider %q", providerStr)
}
