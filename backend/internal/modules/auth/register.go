package auth

import "github.com/gin-gonic/gin"

// Register only provisions a user after a verified email challenge and
// the current server-controlled registration policy authorizes enrollment.
func (h *Handler) Register(c *gin.Context) { h.registerAccount(c) }
