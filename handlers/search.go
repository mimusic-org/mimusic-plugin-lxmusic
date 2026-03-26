//go:build wasip1

package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"mimusic-plugin-lxmusic/engine"
	"mimusic-plugin-lxmusic/source"

	"github.com/mimusic-org/plugin/api/pbplugin"
	"github.com/mimusic-org/plugin/api/plugin"
)

// SearchHandler 搜索处理器
type SearchHandler struct {
	manager *source.Manager
	engine  *engine.Runtime
}

// NewSearchHandler 创建搜索处理器
func NewSearchHandler(manager *source.Manager, engine *engine.Runtime) *SearchHandler {
	return &SearchHandler{
		manager: manager,
		engine:  engine,
	}
}

// HandleSearch 搜索歌曲
// GET /lxmusic/api/search?keyword=xxx&source_id=xxx&page=1
func (h *SearchHandler) HandleSearch(req *http.Request) (*plugin.RouterResponse, error) {
	// 获取查询参数
	keyword := req.URL.Query().Get("keyword")
	sourceID := req.URL.Query().Get("source_id")
	pageStr := req.URL.Query().Get("page")

	if keyword == "" {
		return plugin.ErrorResponse(http.StatusBadRequest, "缺少 keyword 参数"), nil
	}
	if sourceID == "" {
		return plugin.ErrorResponse(http.StatusBadRequest, "缺少 source_id 参数"), nil
	}

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	// 获取音源脚本
	script, err := h.manager.GetSourceScript(sourceID)
	if err != nil {
		return plugin.ErrorResponse(http.StatusNotFound, "音源不存在: "+err.Error()), nil
	}

	// 获取音源信息以确定平台
	sourceInfo := h.manager.GetSource(sourceID)
	if sourceInfo == nil {
		return plugin.ErrorResponse(http.StatusNotFound, "音源不存在"), nil
	}

	// 加载音源配置以获取可用平台
	config, err := h.engine.LoadSource(script)
	if err != nil {
		return plugin.ErrorResponse(http.StatusInternalServerError, "加载音源失败: "+err.Error()), nil
	}

	// 使用第一个可用的平台
	var platform string
	for p := range config.Sources {
		platform = p
		break
	}
	if platform == "" {
		return plugin.ErrorResponse(http.StatusBadRequest, "音源没有可用的平台"), nil
	}

	slog.Info("开始搜索", "keyword", keyword, "sourceID", sourceID, "platform", platform, "page", page)

	// 执行搜索
	result, err := h.engine.Search(script, platform, keyword, page, 30)
	if err != nil {
		slog.Error("搜索失败", "error", err)
		return plugin.ErrorResponse(http.StatusInternalServerError, "搜索失败: "+err.Error()), nil
	}

	// 返回结果
	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"list":   result.List,
			"total":  result.Total,
			"source": result.Source,
		},
	}
	body, _ := json.Marshal(response)
	return &plugin.RouterResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}, nil
}

// ImportSongsRequest 导入歌曲请求
type ImportSongsRequest struct {
	SourceID string       `json:"source_id"`
	Songs    []SongImport `json:"songs"`
}

// SongImport 要导入的歌曲
type SongImport struct {
	Name    string `json:"name"`
	Singer  string `json:"singer"`
	Album   string `json:"album"`
	Source  string `json:"source"`
	MusicID string `json:"music_id"`
	Quality string `json:"quality"`
	Img     string `json:"img"`
}

// ImportResult 导入结果
type ImportResult struct {
	Name    string `json:"name"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// HandleImportSongs 批量导入歌曲
// POST /lxmusic/api/songs/import
func (h *SearchHandler) HandleImportSongs(req *http.Request) (*plugin.RouterResponse, error) {
	var request ImportSongsRequest
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		return plugin.ErrorResponse(http.StatusBadRequest, "无效的请求参数"), nil
	}

	if len(request.Songs) == 0 {
		return plugin.ErrorResponse(http.StatusBadRequest, "请选择至少一首歌曲"), nil
	}

	// 获取音源脚本
	script, err := h.manager.GetSourceScript(request.SourceID)
	if err != nil {
		return plugin.ErrorResponse(http.StatusNotFound, "音源不存在: "+err.Error()), nil
	}

	hostFunctions := pbplugin.NewHostFunctions()

	var results []ImportResult
	successCount := 0
	failedCount := 0

	// 逐个处理歌曲（WASM 单线程）
	for _, song := range request.Songs {
		result := ImportResult{Name: song.Name}

		// 构建 musicInfo
		musicInfo := map[string]interface{}{
			"name":    song.Name,
			"singer":  song.Singer,
			"album":   song.Album,
			"source":  song.Source,
			"musicId": song.MusicID,
		}

		// 获取播放 URL
		quality := song.Quality
		if quality == "" {
			quality = "320k"
		}

		musicUrl, err := h.engine.GetMusicUrl(script, song.Source, musicInfo, quality)
		if err != nil {
			slog.Error("获取播放URL失败", "name", song.Name, "error", err)
			result.Success = false
			result.Error = "获取播放URL失败: " + err.Error()
			results = append(results, result)
			failedCount++
			continue
		}

		if musicUrl == "" {
			result.Success = false
			result.Error = "获取到的播放URL为空"
			results = append(results, result)
			failedCount++
			continue
		}

		// 调用主程序 API 添加歌曲
		addSongBody := map[string]interface{}{
			"title":     song.Name,
			"artist":    song.Singer,
			"album":     song.Album,
			"url":       musicUrl,
			"cover_url": song.Img,
		}
		bodyBytes, _ := json.Marshal(addSongBody)

		resp, err := hostFunctions.CallRouter(req.Context(), &pbplugin.CallRouterRequest{
			Method: "POST",
			Path:   "/api/v1/songs/remote",
			Body:   bodyBytes,
		})

		if err != nil {
			slog.Error("调用主程序API失败", "name", song.Name, "error", err)
			result.Success = false
			result.Error = "添加歌曲失败: " + err.Error()
			results = append(results, result)
			failedCount++
			continue
		}

		if !resp.Success {
			result.Success = false
			result.Error = "添加歌曲失败: " + resp.Message
			results = append(results, result)
			failedCount++
			continue
		}

		result.Success = true
		results = append(results, result)
		successCount++
		slog.Info("歌曲导入成功", "name", song.Name)
	}

	// 返回结果统计
	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"total":   len(request.Songs),
			"success": successCount,
			"failed":  failedCount,
			"results": results,
		},
	}
	body, _ := json.Marshal(response)
	return &plugin.RouterResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}, nil
}

// GetMusicUrlRequest 获取播放URL请求
type GetMusicUrlRequest struct {
	SourceID string `json:"source_id"`
	Source   string `json:"source"`
	MusicID  string `json:"music_id"`
	Quality  string `json:"quality"`
}

// HandleGetMusicUrl 获取歌曲播放URL
// POST /lxmusic/api/songs/get-url
func (h *SearchHandler) HandleGetMusicUrl(req *http.Request) (*plugin.RouterResponse, error) {
	var request GetMusicUrlRequest
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		return plugin.ErrorResponse(http.StatusBadRequest, "无效的请求参数"), nil
	}

	// 获取音源脚本
	script, err := h.manager.GetSourceScript(request.SourceID)
	if err != nil {
		return plugin.ErrorResponse(http.StatusNotFound, "音源不存在: "+err.Error()), nil
	}

	// 构建 musicInfo
	musicInfo := map[string]interface{}{
		"source":  request.Source,
		"musicId": request.MusicID,
	}

	quality := request.Quality
	if quality == "" {
		quality = "320k"
	}

	// 获取播放 URL
	musicUrl, err := h.engine.GetMusicUrl(script, request.Source, musicInfo, quality)
	if err != nil {
		return plugin.ErrorResponse(http.StatusInternalServerError, "获取播放URL失败: "+err.Error()), nil
	}

	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"url": musicUrl,
		},
	}
	body, _ := json.Marshal(response)
	return &plugin.RouterResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}, nil
}
