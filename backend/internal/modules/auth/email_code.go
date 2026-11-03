package auth

import "github.com/gin-gonic/gin"

// SendEmailCode enforces rate limits, email proof and the registration policy.
func (h *Handler) SendEmailCode(c *gin.Context) { h.sendEmailChallenge(c) }
