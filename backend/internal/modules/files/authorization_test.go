package files

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func TestReadonlyCanListFilesButCannotUploadOrDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &shop.Shop{}, &FileRecord{}))
	user := admin.AdminUser{TenantID: 27, Username: admin.NewInternalUsername(), Role: adminperm.RoleReadonly, Status: admin.StatusActive}
	require.NoError(t, db.Create(&user).Error)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, user.ID.String())
		c.Set(ctxkey.TenantID, user.TenantID)
		c.Next()
	})
	h := &Handler{Svc: &Service{DB: db}}
	r.GET("/files", h.List)
	r.POST("/files/upload", h.Upload)
	r.DELETE("/files/:id", h.Delete)

	list := httptest.NewRecorder()
	r.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/files", nil))
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())

	upload := httptest.NewRecorder()
	r.ServeHTTP(upload, httptest.NewRequest(http.MethodPost, "/files/upload", nil))
	require.Equal(t, http.StatusForbidden, upload.Code, upload.Body.String())

	remove := httptest.NewRecorder()
	r.ServeHTTP(remove, httptest.NewRequest(http.MethodDelete, "/files/"+uuid.NewString(), nil))
	require.Equal(t, http.StatusForbidden, remove.Code, remove.Body.String())
}
