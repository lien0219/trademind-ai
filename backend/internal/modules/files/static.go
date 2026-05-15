package files

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
)

// StaticHandler serves uploaded local files from configured storage.local_root.
type StaticHandler struct {
	Settings *settings.Service
}

// Serve GET /static/*filepath
func (h *StaticHandler) Serve(c *gin.Context) {
	if h == nil || h.Settings == nil {
		c.Status(http.StatusNotFound)
		return
	}
	m, err := h.Settings.PlainByGroup(c.Request.Context(), 0, "storage")
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	if strings.ToLower(strings.TrimSpace(m["kind"])) != "local" {
		c.Status(http.StatusNotFound)
		return
	}
	root := strings.TrimSpace(m["local_root"])
	if root == "" {
		root = "data/uploads"
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	rel := strings.TrimPrefix(c.Param("filepath"), "/")
	rel = strings.ReplaceAll(rel, "\\", "/")
	if rel == "" || strings.Contains(rel, "..") {
		c.Status(http.StatusNotFound)
		return
	}
	full := filepath.Join(absRoot, filepath.FromSlash(rel))
	absFull, err := filepath.Abs(full)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	sep := string(os.PathSeparator)
	if !strings.HasPrefix(absFull, absRoot+sep) && absFull != absRoot {
		c.Status(http.StatusNotFound)
		return
	}
	st, err := os.Stat(absFull)
	if err != nil || st.IsDir() {
		c.Status(http.StatusNotFound)
		return
	}
	http.ServeFile(c.Writer, c.Request, absFull)
}
