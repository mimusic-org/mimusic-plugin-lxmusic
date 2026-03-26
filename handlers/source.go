//go:build wasip1

package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"mimusic-plugin-lxmusic/source"

	"github.com/mimusic-org/plugin/api/plugin"
)

// SourceHandler 音源管理处理器
type SourceHandler struct {
	manager *source.Manager
}

// NewSourceHandler 创建音源处理器
func NewSourceHandler(manager *source.Manager) *SourceHandler {
	return &SourceHandler{
		manager: manager,
	}
}

// HandleListSources 列出所有音源
// GET /lxmusic/api/sources
func (h *SourceHandler) HandleListSources(req *http.Request) (*plugin.RouterResponse, error) {
	sources := h.manager.ListSources()

	// 构建响应（不包含 Script 字段）
	type SourceItem struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
		Author      string `json:"author"`
		Filename    string `json:"filename"`
		ImportedAt  string `json:"imported_at"`
	}

	items := make([]SourceItem, 0, len(sources))
	for _, s := range sources {
		items = append(items, SourceItem{
			ID:          s.ID,
			Name:        s.Name,
			Version:     s.Version,
			Description: s.Description,
			Author:      s.Author,
			Filename:    s.Filename,
			ImportedAt:  s.ImportedAt,
		})
	}

	return plugin.SuccessResponse(items), nil
}

// HandleImportSource 导入音源
// POST /lxmusic/api/sources/import
func (h *SourceHandler) HandleImportSource(req *http.Request) (*plugin.RouterResponse, error) {
	// 解析 multipart form，32MB 限制
	if err := req.ParseMultipartForm(32 << 20); err != nil {
		slog.Error("解析 multipart form 失败", "error", err)
		return plugin.ErrorResponse(http.StatusBadRequest, "无效的请求格式: "+err.Error()), nil
	}

	// 获取上传的文件
	file, header, err := req.FormFile("file")
	if err != nil {
		slog.Error("获取上传文件失败", "error", err)
		return plugin.ErrorResponse(http.StatusBadRequest, "请上传文件"), nil
	}
	defer file.Close()

	// 读取文件内容
	content, err := io.ReadAll(file)
	if err != nil {
		slog.Error("读取文件内容失败", "error", err)
		return plugin.ErrorResponse(http.StatusInternalServerError, "读取文件失败"), nil
	}

	filename := header.Filename
	slog.Info("收到文件上传", "filename", filename, "size", len(content))

	// 根据文件扩展名判断类型
	if strings.HasSuffix(strings.ToLower(filename), ".js") {
		// 导入单个 JS 文件
		info, err := h.manager.ImportFromJS(filename, content)
		if err != nil {
			slog.Error("导入 JS 文件失败", "error", err)
			return plugin.ErrorResponse(http.StatusBadRequest, "导入失败: "+err.Error()), nil
		}
		return plugin.SuccessResponse(info), nil

	} else if strings.HasSuffix(strings.ToLower(filename), ".zip") {
		// 导入 ZIP 文件
		sources, err := h.manager.ImportFromZIP(content)
		if err != nil {
			slog.Error("导入 ZIP 文件失败", "error", err)
			return plugin.ErrorResponse(http.StatusBadRequest, "导入失败: "+err.Error()), nil
		}
		return plugin.SuccessResponse(sources), nil

	} else {
		return plugin.ErrorResponse(http.StatusBadRequest, "不支持的文件格式，请上传 .js 或 .zip 文件"), nil
	}
}

// HandleDeleteSource 删除音源
// DELETE /lxmusic/api/sources?id=xxx
func (h *SourceHandler) HandleDeleteSource(req *http.Request) (*plugin.RouterResponse, error) {
	// 从 query parameter 获取 id
	id := req.URL.Query().Get("id")
	if id == "" {
		return plugin.ErrorResponse(http.StatusBadRequest, "缺少 id 参数"), nil
	}

	if err := h.manager.DeleteSource(id); err != nil {
		slog.Error("删除音源失败", "id", id, "error", err)
		return plugin.ErrorResponse(http.StatusNotFound, "删除失败: "+err.Error()), nil
	}

	// 返回成功消息
	response := map[string]interface{}{
		"success": true,
		"message": "删除成功",
	}
	body, _ := json.Marshal(response)
	return &plugin.RouterResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}, nil
}
