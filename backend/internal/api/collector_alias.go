package api

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"gorm.io/gorm"
)

func registerCollectorAliases(r gin.IRouter, h *collect.Handler, db *gorm.DB, auth gin.HandlerFunc) {
	if r == nil || h == nil || auth == nil {
		return
	}
	collect.RegisterAliases(r, h, db, auth)
}
