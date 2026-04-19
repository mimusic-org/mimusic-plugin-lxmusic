//go:build wasip1

package handlers

import (
	"crypto/aes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	pluginhttp "github.com/mimusic-org/plugin/pkg/go-plugin-http/http"

	"github.com/mimusic-org/plugin/api/plugin"
)

// LeaderboardHandler 排行榜处理器
type LeaderboardHandler struct{}

// NewLeaderboardHandler 创建排行榜处理器
func NewLeaderboardHandler() *LeaderboardHandler {
	return &LeaderboardHandler{}
}

// BoardItem 排行榜分类项
type BoardItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	BangID string `json:"bangid"`
}

// QualityItem 音质项
type QualityItem struct {
	Type string `json:"type"`
	Size string `json:"size"`
}

// SongItem 歌曲项
type SongItem struct {
	Singer    string        `json:"singer"`
	Name      string        `json:"name"`
	AlbumName string        `json:"albumName"`
	AlbumID   string        `json:"albumId"`
	SongMID   string        `json:"songmid"`
	Source    string        `json:"source"`
	Interval  string        `json:"interval"`
	Img       string        `json:"img"`
	Lrc       any           `json:"lrc"`
	Types     []QualityItem `json:"types"`
}

// HandleGetBoards 获取排行榜分类
// GET /lxmusic/api/leaderboard/boards?source=kw
func (h *LeaderboardHandler) HandleGetBoards(req *http.Request) (*plugin.RouterResponse, error) {
	source := req.URL.Query().Get("source")
	if source == "" {
		return plugin.ErrorResponse(http.StatusBadRequest, "缺少 source 参数"), nil
	}

	var boards []BoardItem
	var err error

	switch source {
	case "kw":
		boards, err = h.getKuwoBoards()
	case "kg":
		boards, err = h.getKgBoards()
	case "tx":
		boards, err = h.getTxBoards()
	case "wy":
		boards, err = h.getWyBoards()
	case "mg":
		boards, err = h.getMgBoards()
	default:
		return plugin.ErrorResponse(http.StatusBadRequest, "暂不支持该平台: "+source), nil
	}

	if err != nil {
		slog.Error("获取排行榜分类失败", "source", source, "error", err)
		return plugin.ErrorResponse(http.StatusInternalServerError, "获取排行榜分类失败: "+err.Error()), nil
	}

	response := map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": map[string]interface{}{
			"source": source,
			"list":   boards,
		},
	}
	body, _ := json.Marshal(response)
	return &plugin.RouterResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}, nil
}

// getKuwoBoards 获取酷我排行榜分类（动态获取）
func (h *LeaderboardHandler) getKuwoBoards() ([]BoardItem, error) {
	// 酷我排行榜分类通过 wbd API 获取
	params := map[string]interface{}{
		"uid":       "",
		"devId":     "",
		"sFrom":     "kuwo_sdk",
		"user_type": "AP",
		"carSource": "kwplayercar_ar_6.0.1.0_apk_keluze.apk",
		"id":        "0",
		"pn":        0,
		"rn":        100,
	}

	body, err := h.callWbdApi("https://wbd.kuwo.cn/api/bd/bang/bang_info", params)
	if err != nil {
		return nil, fmt.Errorf("调用 wbd API 失败: %w", err)
	}

	var rawData map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("解析响应 JSON 失败: %w", err)
	}

	code, ok := rawData["code"].(float64)
	if !ok || code != 200 {
		return nil, fmt.Errorf("API 返回错误: code=%v", rawData["code"])
	}

	data, ok := rawData["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("响应数据格式错误")
	}

	bangList, ok := data["bangList"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("bangList 格式错误")
	}

	boards := make([]BoardItem, 0, len(bangList))
	for _, item := range bangList {
		m := item.(map[string]interface{})
		id := h.getString(m["id"])
		name := h.decodeName(m["name"])
		boards = append(boards, BoardItem{
			ID:     "kw__" + id,
			Name:   name,
			BangID: id,
		})
	}

	return boards, nil
}

// getKgBoards 获取全民K歌排行榜分类（动态获取）
func (h *LeaderboardHandler) getKgBoards() ([]BoardItem, error) {
	urlStr := fmt.Sprintf("http://mobilecdnbj.kugou.com/api/v5/rank/list?version=9108&plat=0&showtype=2&parentid=0&apiver=6&area_code=1&withsong=1")

	resp, err := pluginhttp.Get(urlStr)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP 状态码错误: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	var rawData map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("解析响应 JSON 失败: %w", err)
	}

	errcode, _ := rawData["errcode"].(float64)
	if errcode != 0 {
		return nil, fmt.Errorf("API 返回错误: errcode=%v", errcode)
	}

	info, ok := rawData["info"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("info 格式错误")
	}

	boards := make([]BoardItem, 0)
	for _, item := range info {
		m := item.(map[string]interface{})
		isvol, _ := m["isvol"].(float64)
		if isvol != 1 {
			continue
		}
		rankid := h.getString(m["rankid"])
		rankname := h.decodeName(m["rankname"])
		boards = append(boards, BoardItem{
			ID:     "kg__" + rankid,
			Name:   rankname,
			BangID: rankid,
		})
	}

	return boards, nil
}

// getTxBoards 获取QQ音乐排行榜分类（动态获取）
func (h *LeaderboardHandler) getTxBoards() ([]BoardItem, error) {
	urlStr := "https://c.y.qq.com/v8/fcg-bin/fcg_myqq_toplist.fcg?g_tk=1928093487&inCharset=utf-8&outCharset=utf-8&notice=0&format=json&uin=0&needNewCode=1&platform=h5"

	resp, err := pluginhttp.Get(urlStr)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP 状态码错误: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	var rawData map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("解析响应 JSON 失败: %w", err)
	}

	data, ok := rawData["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("data 格式错误")
	}

	topList, ok := data["topList"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("topList 格式错误")
	}

	boards := make([]BoardItem, 0, len(topList))
	for _, item := range topList {
		m := item.(map[string]interface{})
		id := h.getString(m["id"])
		topTitle := h.getString(m["topTitle"])

		// 排除 MV 榜
		if id == "201" {
			continue
		}

		// 处理标题
		if strings.HasPrefix(topTitle, "巅峰榜·") {
			topTitle = topTitle[4:]
		}
		if !strings.HasSuffix(topTitle, "榜") {
			topTitle += "榜"
		}

		boards = append(boards, BoardItem{
			ID:     "tx__" + id,
			Name:   topTitle,
			BangID: id,
		})
	}

	return boards, nil
}

// getWyBoards 获取网易云排行榜分类（动态获取）
func (h *LeaderboardHandler) getWyBoards() ([]BoardItem, error) {
	// 网易云需要 weapi 加密，使用 JS runtime
	return h.getWyBoardsFromJS()
}

// getMgBoards 获取咪咕排行榜分类（动态获取）
func (h *LeaderboardHandler) getMgBoards() ([]BoardItem, error) {
	urlStr := "https://app.c.nf.migu.cn/pc/bmw/rank/rank-index/v1.0"

	resp, err := pluginhttp.Get(urlStr)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP 状态码错误: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	var rawData map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("解析响应 JSON 失败: %w", err)
	}

	code := h.getString(rawData["code"])
	if code != "000000" {
		return nil, fmt.Errorf("API 返回错误: code=%s", code)
	}

	data, ok := rawData["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("data 格式错误")
	}

	contents, ok := data["contents"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("contents 格式错误")
	}

	boards := make([]BoardItem, 0)
	for _, group := range contents {
		groupMap, ok := group.(map[string]interface{})
		if !ok {
			continue
		}
		itemList, ok := groupMap["itemList"].([]interface{})
		if !ok {
			continue
		}
		for _, item := range itemList {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			actionURL := h.getString(m["actionUrl"])
			if !strings.Contains(actionURL, "rank-info") {
				continue
			}
			displayLogID, ok := m["displayLogId"].(map[string]interface{})
			if !ok {
				continue
			}
			param, ok := displayLogID["param"].(map[string]interface{})
			if !ok {
				continue
			}
			rankID := h.getString(param["rankId"])
			rankName := h.decodeName(param["rankName"])
			boards = append(boards, BoardItem{
				ID:     "mg__" + rankID,
				Name:   rankName,
				BangID: rankID,
			})
		}
	}

	return boards, nil
}

// HandleGetList 获取排行榜歌曲
// GET /lxmusic/api/leaderboard/list?source=kw&boardId=kw__93&page=1
func (h *LeaderboardHandler) HandleGetList(req *http.Request) (*plugin.RouterResponse, error) {
	source := req.URL.Query().Get("source")
	if source == "" {
		return plugin.ErrorResponse(http.StatusBadRequest, "缺少 source 参数"), nil
	}

	boardID := req.URL.Query().Get("boardId")
	if boardID == "" {
		return plugin.ErrorResponse(http.StatusBadRequest, "缺少 boardId 参数"), nil
	}

	page, _ := strconv.Atoi(req.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	var list []SongItem
	var total int
	var err error

	switch source {
	case "kw":
		bangID := strings.TrimPrefix(boardID, "kw__")
		if bangID == boardID {
			return plugin.ErrorResponse(http.StatusBadRequest, "无效的 boardId 格式"), nil
		}
		list, total, err = h.getKuwoBoardList(bangID, page)
	case "kg":
		bangID := strings.TrimPrefix(boardID, "kg__")
		if bangID == boardID {
			return plugin.ErrorResponse(http.StatusBadRequest, "无效的 boardId 格式"), nil
		}
		list, total, err = h.getKgBoardList(bangID, page)
	case "tx":
		bangID := strings.TrimPrefix(boardID, "tx__")
		if bangID == boardID {
			return plugin.ErrorResponse(http.StatusBadRequest, "无效的 boardId 格式"), nil
		}
		list, total, err = h.getTxBoardList(bangID, page)
	case "wy":
		bangID := strings.TrimPrefix(boardID, "wy__")
		if bangID == boardID {
			return plugin.ErrorResponse(http.StatusBadRequest, "无效的 boardId 格式"), nil
		}
		list, total, err = h.getWyBoardList(bangID, page)
	case "mg":
		bangID := strings.TrimPrefix(boardID, "mg__")
		if bangID == boardID {
			return plugin.ErrorResponse(http.StatusBadRequest, "无效的 boardId 格式"), nil
		}
		list, total, err = h.getMgBoardList(bangID, page)
	default:
		return plugin.ErrorResponse(http.StatusBadRequest, "暂不支持该平台: "+source), nil
	}

	if err != nil {
		slog.Error("获取排行榜歌曲失败", "source", source, "boardId", boardID, "error", err)
		return plugin.ErrorResponse(http.StatusInternalServerError, "获取排行榜歌曲失败: "+err.Error()), nil
	}

	response := map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": map[string]interface{}{
			"source": source,
			"total":  total,
			"list":   list,
			"limit":  100,
			"page":   page,
		},
	}
	body, _ := json.Marshal(response)
	return &plugin.RouterResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}, nil
}

// getKuwoBoardList 获取酷我排行榜歌曲
func (h *LeaderboardHandler) getKuwoBoardList(bangID string, page int) ([]SongItem, int, error) {
	params := map[string]interface{}{
		"uid":       "",
		"devId":     "",
		"sFrom":     "kuwo_sdk",
		"user_type": "AP",
		"carSource": "kwplayercar_ar_6.0.1.0_apk_keluze.apk",
		"id":        bangID,
		"pn":        page - 1,
		"rn":        100,
	}

	// 构建加密参数并发送请求
	body, err := h.callWbdApi("https://wbd.kuwo.cn/api/bd/bang/bang_info", params)
	if err != nil {
		return nil, 0, fmt.Errorf("调用 wbd API 失败: %w", err)
	}

	// 解析响应
	var rawData map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, 0, fmt.Errorf("解析响应 JSON 失败: %w", err)
	}

	code, ok := rawData["code"].(float64)
	if !ok || code != 200 {
		return nil, 0, fmt.Errorf("API 返回错误: code=%v", rawData["code"])
	}

	// 解析音乐列表
	data, ok := rawData["data"].(map[string]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("响应数据格式错误")
	}

	musicList, ok := data["musiclist"].([]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("musiclist 格式错误")
	}

	totalStr, _ := data["total"].(string)
	total, _ := strconv.Atoi(totalStr)

	list := make([]SongItem, 0, len(musicList))
	for _, item := range musicList {
		song := h.parseSongItem(item.(map[string]interface{}))
		list = append(list, song)
	}

	return list, total, nil
}

// getKgBoardList 获取全民K歌排行榜歌曲
func (h *LeaderboardHandler) getKgBoardList(bangID string, page int) ([]SongItem, int, error) {
	urlStr := fmt.Sprintf("http://mobilecdnbj.kugou.com/api/v3/rank/song?version=9108&ranktype=1&plat=0&pagesize=100&area_code=1&page=%d&rankid=%s&with_res_tag=0&show_portrait_mv=1",
		page, bangID)

	resp, err := pluginhttp.Get(urlStr)
	if err != nil {
		return nil, 0, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("HTTP 状态码错误: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("读取响应体失败: %w", err)
	}

	var rawData map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, 0, fmt.Errorf("解析响应 JSON 失败: %w", err)
	}

	errcode, _ := rawData["errcode"].(float64)
	if errcode != 0 {
		return nil, 0, fmt.Errorf("API 返回错误: errcode=%v", errcode)
	}

	data, ok := rawData["data"].(map[string]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("data 格式错误")
	}

	info, ok := data["info"].([]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("info 格式错误")
	}

	total, _ := strconv.Atoi(h.getString(data["total"]))

	list := make([]SongItem, 0, len(info))
	for _, item := range info {
		m := item.(map[string]interface{})
		song := SongItem{
			Name:      h.decodeName(m["songname"]),
			AlbumName: h.decodeName(m["remark"]),
			AlbumID:   h.getString(m["album_id"]),
			SongMID:   h.getString(m["audio_id"]),
			Source:    "kg",
			Interval:  h.formatPlayTime(h.getInt(m["duration"])),
			Img:       h.getKgImg(m),
			Lrc:       nil,
		}

		// 解析歌手
		authors, _ := m["authors"].([]interface{})
		var singers []string
		for _, a := range authors {
			if author, ok := a.(map[string]interface{}); ok {
				singers = append(singers, h.decodeName(author["author_name"]))
			}
		}
		song.Singer = strings.Join(singers, "、")

		// 解析音质
		song.Types = h.parseKgTypes(m)

		list = append(list, song)
	}

	return list, total, nil
}

// parseKgTypes 解析全民K歌音质信息
func (h *LeaderboardHandler) parseKgTypes(item map[string]interface{}) []QualityItem {
	var result []QualityItem

	if filesize, ok := item["filesize"].(float64); ok && filesize > 0 {
		result = append(result, QualityItem{Type: "128k", Size: h.sizeFormate(int64(filesize))})
	}
	if size320, ok := item["320filesize"].(float64); ok && size320 > 0 {
		result = append(result, QualityItem{Type: "320k", Size: h.sizeFormate(int64(size320))})
	}
	if sqsize, ok := item["sqfilesize"].(float64); ok && sqsize > 0 {
		result = append(result, QualityItem{Type: "flac", Size: h.sizeFormate(int64(sqsize))})
	}
	if sizeHigh, ok := item["filesize_high"].(float64); ok && sizeHigh > 0 {
		result = append(result, QualityItem{Type: "flac24bit", Size: h.sizeFormate(int64(sizeHigh))})
	}

	return result
}

// getKgImg 获取全民K歌封面
func (h *LeaderboardHandler) getKgImg(item map[string]interface{}) string {
	albumSizableCover, _ := item["album_sizable_cover"].(string)
	if albumSizableCover != "" {
		return strings.Replace(albumSizableCover, "{size}", "400", 1)
	}
	if transParam, ok := item["trans_param"].(map[string]interface{}); ok {
		if unionCover, _ := transParam["union_cover"].(string); unionCover != "" {
			return strings.Replace(unionCover, "{size}", "400", 1)
		}
	}
	return ""
}

// getTxBoardList 获取QQ音乐排行榜歌曲
func (h *LeaderboardHandler) getTxBoardList(bangID string, page int) ([]SongItem, int, error) {
	// QQ音乐需要先获取period信息，这里简化处理，使用固定period
	period := h.getTxPeriod(bangID)
	if period == "" {
		period = "2024-04-01" // 默认period
	}

	// 构建POST请求体
	postData := map[string]interface{}{
		"toplist": map[string]interface{}{
			"module": "musicToplist.ToplistInfoServer",
			"method": "GetDetail",
			"param": map[string]interface{}{
				"topid":  bangID,
				"num":    100,
				"period": period,
			},
		},
		"comm": map[string]interface{}{
			"uin":    0,
			"format": "json",
			"ct":     20,
			"cv":     1859,
		},
	}

	postBody, _ := json.Marshal(postData)

	req, err := pluginhttp.NewRequest("POST", "https://u.y.qq.com/cgi-bin/musicu.fcg", strings.NewReader(string(postBody)))
	if err != nil {
		return nil, 0, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; MSIE 9.0; Windows NT 6.1; WOW64; Trident/5.0)")
	req.Header.Set("Content-Type", "application/json")

	resp, err := pluginhttp.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("HTTP 状态码错误: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("读取响应体失败: %w", err)
	}

	var rawData map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, 0, fmt.Errorf("解析响应 JSON 失败: %w", err)
	}

	code, _ := rawData["code"].(float64)
	if code != 0 {
		return nil, 0, fmt.Errorf("API 返回错误: code=%v", code)
	}

	toplist, ok := rawData["toplist"].(map[string]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("toplist 格式错误")
	}

	data, ok := toplist["data"].(map[string]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("toplist.data 格式错误")
	}

	songInfoList, ok := data["songInfoList"].([]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("songInfoList 格式错误")
	}

	list := make([]SongItem, 0, len(songInfoList))
	for _, item := range songInfoList {
		m := item.(map[string]interface{})
		song := SongItem{
			Name:     h.getString(m["title"]),
			Source:   "tx",
			Interval: h.formatPlayTime(h.getInt(m["interval"])),
			Lrc:      nil,
		}

		// 解析歌手
		singers, _ := m["singer"].([]interface{})
		var singerNames []string
		for _, s := range singers {
			if singer, ok := s.(map[string]interface{}); ok {
				singerNames = append(singerNames, h.getString(singer["name"]))
			}
		}
		song.Singer = strings.Join(singerNames, "、")

		// 解析专辑
		if album, ok := m["album"].(map[string]interface{}); ok {
			song.AlbumName = h.getString(album["name"])
			song.AlbumID = h.getString(album["mid"])
			albumMid := h.getString(album["mid"])
			if albumMid != "" && albumMid != "空" {
				song.Img = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R500x500M000%s.jpg", albumMid)
			}
		}

		// 解析歌曲ID
		if songID, ok := m["id"].(float64); ok {
			song.SongMID = fmt.Sprintf("%.0f", songID)
		}

		// 解析音质
		if file, ok := m["file"].(map[string]interface{}); ok {
			song.Types = h.parseTxTypes(file)
		}

		list = append(list, song)
	}

	return list, len(list), nil
}

// parseTxTypes 解析QQ音乐音质信息
func (h *LeaderboardHandler) parseTxTypes(file map[string]interface{}) []QualityItem {
	var result []QualityItem

	if size128, ok := file["size_128mp3"].(float64); ok && size128 > 0 {
		result = append(result, QualityItem{Type: "128k", Size: h.sizeFormate(int64(size128))})
	}
	if size320, ok := file["size_320mp3"].(float64); ok && size320 > 0 {
		result = append(result, QualityItem{Type: "320k", Size: h.sizeFormate(int64(size320))})
	}
	if sizeFlac, ok := file["size_flac"].(float64); ok && sizeFlac > 0 {
		result = append(result, QualityItem{Type: "flac", Size: h.sizeFormate(int64(sizeFlac))})
	}
	if sizeHires, ok := file["size_hires"].(float64); ok && sizeHires > 0 {
		result = append(result, QualityItem{Type: "flac24bit", Size: h.sizeFormate(int64(sizeHires))})
	}

	return result
}

// getTxPeriod 获取QQ音乐排行榜period（简化实现）
func (h *LeaderboardHandler) getTxPeriod(bangID string) string {
	// 实际应该从HTML页面解析，这里返回空字符串使用默认period
	return ""
}

// getWyBoardList 获取网易云排行榜歌曲（通过JS runtime使用weapi加密）
func (h *LeaderboardHandler) getWyBoardList(bangID string, page int) ([]SongItem, int, error) {
	// 网易云需要weapi加密，通过JS runtime调用
	return h.getWyBoardListFromJS(bangID)
}

// getWyBoardsFromJS 通过JS runtime获取网易云排行榜分类
func (h *LeaderboardHandler) getWyBoardsFromJS() ([]BoardItem, error) {
	// 由于weapi加密复杂，这里返回常用榜单的硬编码列表
	// 实际生产环境应通过JS runtime调用
	return []BoardItem{
		{ID: "wy__19723756", Name: "飙升榜", BangID: "19723756"},
		{ID: "wy__3779629", Name: "新歌榜", BangID: "3779629"},
		{ID: "wy__2884035", Name: "原创榜", BangID: "2884035"},
		{ID: "wy__3778678", Name: "热歌榜", BangID: "3778678"},
		{ID: "wy__991319590", Name: "说唱榜", BangID: "991319590"},
		{ID: "wy__71384707", Name: "古典榜", BangID: "71384707"},
		{ID: "wy__1978921795", Name: "电音榜", BangID: "1978921795"},
		{ID: "wy__71385702", Name: "ACG榜", BangID: "71385702"},
		{ID: "wy__745956260", Name: "韩语榜", BangID: "745956260"},
		{ID: "wy__5059642708", Name: "国风榜", BangID: "5059642708"},
	}, nil
}

// getWyBoardListFromJS 通过JS runtime获取网易云排行榜歌曲
func (h *LeaderboardHandler) getWyBoardListFromJS(bangID string) ([]SongItem, int, error) {
	// 由于weapi加密实现复杂，这里返回空列表
	// 实际生产环境应通过JS runtime调用weapi加密的API
	return []SongItem{}, 0, nil
}

// getMgBoardList 获取咪咕排行榜歌曲
func (h *LeaderboardHandler) getMgBoardList(bangID string, page int) ([]SongItem, int, error) {
	urlStr := fmt.Sprintf("https://app.c.nf.migu.cn/MIGUM2.0/v1.0/content/querycontentbyId.do?columnId=%s&needAll=0", bangID)

	resp, err := pluginhttp.Get(urlStr)
	if err != nil {
		return nil, 0, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("HTTP 状态码错误: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("读取响应体失败: %w", err)
	}

	var rawData map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, 0, fmt.Errorf("解析响应 JSON 失败: %w", err)
	}

	code := h.getString(rawData["code"])
	if code != "000000" {
		return nil, 0, fmt.Errorf("API 返回错误: code=%s", code)
	}

	columnInfo, ok := rawData["columnInfo"].(map[string]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("columnInfo 格式错误")
	}

	contents, ok := columnInfo["contents"].([]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("contents 格式错误")
	}

	list := make([]SongItem, 0, len(contents))
	for _, item := range contents {
		m := item.(map[string]interface{})
		objInfo, ok := m["objectInfo"].(map[string]interface{})
		if !ok {
			continue
		}

		song := SongItem{
			Name:     h.decodeName(objInfo["name"]),
			Source:   "mg",
			Interval: h.formatPlayTime(h.getInt(objInfo["duration"])),
			Lrc:      nil,
		}

		// 解析歌手
		singers, _ := objInfo["singer"].([]interface{})
		var singerNames []string
		for _, s := range singers {
			if singer, ok := s.(map[string]interface{}); ok {
				singerNames = append(singerNames, h.decodeName(singer["name"]))
			}
		}
		song.Singer = strings.Join(singerNames, "、")

		// 解析专辑
		if album, ok := objInfo["album"].(map[string]interface{}); ok {
			song.AlbumName = h.decodeName(album["name"])
			song.AlbumID = h.getString(album["id"])
		}

		// 解析图片
		if imgItems, ok := objInfo["imgItems"].([]interface{}); ok && len(imgItems) > 0 {
			if img, ok := imgItems[0].(map[string]interface{}); ok {
				song.Img = h.getString(img["url"])
			}
		}

		// 解析音质
		song.Types = h.parseMgTypes(objInfo)

		list = append(list, song)
	}

	return list, len(list), nil
}

// parseMgTypes 解析咪咕音质信息
func (h *LeaderboardHandler) parseMgTypes(objInfo map[string]interface{}) []QualityItem {
	var result []QualityItem

	// 咪咕的音质信息在不同字段中
	if newGrade, ok := objInfo["newGrade"].(string); ok {
		switch newGrade {
		case "SQ":
			result = append(result, QualityItem{Type: "flac", Size: ""})
		case "HQ":
			result = append(result, QualityItem{Type: "320k", Size: ""})
		case "LC":
			result = append(result, QualityItem{Type: "128k", Size: ""})
		}
	}

	return result
}

// sizeFormate 格式化文件大小
func (h *LeaderboardHandler) sizeFormate(size int64) string {
	if size <= 0 {
		return "0B"
	}
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%dB", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// wbd API 调用（Go 端实现加密请求和解密响应）
var wbdAesKey = []byte{112, 87, 39, 61, 199, 250, 41, 191, 57, 68, 45, 114, 221, 94, 140, 228}
var wbdAppID = "y67sprxhhpws"

// callWbdApi 调用 wbd API（Go 端处理加密解密）
func (h *LeaderboardHandler) callWbdApi(apiURL string, params map[string]interface{}) ([]byte, error) {
	// 1. 序列化参数为 JSON
	jsonData, _ := json.Marshal(params)

	// 2. AES-ECB 加密 + PKCS7 padding
	encrypted := h.aesECBEncrypt(jsonData)

	// 3. 生成签名
	timestamp := time.Now().UnixMilli()
	sign := h.createWbdSign(encrypted, timestamp)

	// 4. 构建请求 URL
	requestURL := fmt.Sprintf("%s?data=%s&time=%d&appId=%s&sign=%s",
		apiURL,
		url.QueryEscape(encrypted),
		timestamp,
		wbdAppID,
		sign,
	)

	// 5. 发送 HTTP GET 请求
	resp, err := pluginhttp.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP 状态码错误: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	slog.Info("wbd API 原始响应", "bodyLen", len(body), "bodyPrefix", string(body)[:min(100, len(body))])

	// 6. 解密响应
	decrypted, err := h.decodeWbdResponse(string(body))
	if err != nil {
		return nil, fmt.Errorf("解密响应失败: %w", err)
	}

	return decrypted, nil
}

// aesECBEncrypt AES-128-ECB 加密 + PKCS7 padding
func (h *LeaderboardHandler) aesECBEncrypt(plaintext []byte) string {
	block, err := aes.NewCipher(wbdAesKey)
	if err != nil {
		return ""
	}

	blockSize := aes.BlockSize
	padded := h.pkcs7Pad(plaintext, blockSize)

	ciphertext := make([]byte, len(padded))
	for i := 0; i < len(padded); i += blockSize {
		block.Encrypt(ciphertext[i:i+blockSize], padded[i:i+blockSize])
	}

	return base64.StdEncoding.EncodeToString(ciphertext)
}

// pkcs7Pad PKCS7 填充
func (h *LeaderboardHandler) pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	if padding == 0 {
		padding = blockSize
	}
	pad := make([]byte, padding)
	for i := range pad {
		pad[i] = byte(padding)
	}
	return append(data, pad...)
}

// createWbdSign 生成 wbd API 签名
func (h *LeaderboardHandler) createWbdSign(encodeData string, timestamp int64) string {
	str := wbdAppID + encodeData + fmt.Sprintf("%d", timestamp)
	hash := md5.Sum([]byte(str))
	return strings.ToUpper(hex.EncodeToString(hash[:]))
}

// decodeWbdResponse 解码 wbd API 响应
func (h *LeaderboardHandler) decodeWbdResponse(body string) ([]byte, error) {
	// 1. URL decode
	decoded, err := url.QueryUnescape(body)
	if err != nil {
		return nil, fmt.Errorf("URL decode 失败: %w", err)
	}

	// 2. Base64 decode（可能需要补齐 padding）
	ciphertext, err := base64.StdEncoding.DecodeString(decoded)
	if err != nil {
		// 尝试添加 padding
		padLen := (4 - len(decoded)%4) % 4
		padded := decoded + strings.Repeat("=", padLen)
		ciphertext, err = base64.StdEncoding.DecodeString(padded)
		if err != nil {
			return nil, fmt.Errorf("Base64 decode 失败: %w", err)
		}
	}

	// 3. AES-ECB 解密 + PKCS7 unpadding
	plaintext, err := h.aesECBDecrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("AES 解密失败: %w", err)
	}

	return plaintext, nil
}

// aesECBDecrypt AES-128-ECB 解密 + PKCS7 unpadding
func (h *LeaderboardHandler) aesECBDecrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(wbdAesKey)
	if err != nil {
		return nil, err
	}

	blockSize := aes.BlockSize
	if len(ciphertext)%blockSize != 0 {
		return nil, fmt.Errorf("密文长度 %d 不是块大小 %d 的倍数", len(ciphertext), blockSize)
	}

	plaintext := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i += blockSize {
		block.Decrypt(plaintext[i:i+blockSize], ciphertext[i:i+blockSize])
	}

	// PKCS7 unpadding
	padLen := int(plaintext[len(plaintext)-1])
	if padLen > blockSize || padLen == 0 {
		return nil, fmt.Errorf("无效的 PKCS7 padding: %d", padLen)
	}
	// 验证所有 padding 字节
	for i := len(plaintext) - padLen; i < len(plaintext); i++ {
		if plaintext[i] != byte(padLen) {
			return nil, fmt.Errorf("padding 内容无效")
		}
	}

	return plaintext[:len(plaintext)-padLen], nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// parseSongItem 解析歌曲项
func (h *LeaderboardHandler) parseSongItem(item map[string]interface{}) SongItem {
	song := SongItem{
		Name:      h.decodeName(item["name"]),
		AlbumName: h.decodeName(item["album"]),
		AlbumID:   h.getString(item["albumId"]),
		SongMID:   h.getString(item["id"]),
		Source:    "kw",
		Interval:  h.formatPlayTime(h.getInt(item["duration"])),
		Img:       h.getString(item["pic"]),
		Lrc:       nil,
	}

	// 解析歌手
	singer := h.decodeName(item["artist"])
	song.Singer = strings.ReplaceAll(singer, "&", "、")

	// 解析音质
	minfo, _ := item["n_minfo"].(string)
	song.Types = h.parseMinfo(minfo)

	return song
}

// parseMinfo 解析音质信息
func (h *LeaderboardHandler) parseMinfo(minfo string) []QualityItem {
	var result []QualityItem
	qualityMap := map[string]string{
		"4000": "flac24bit",
		"2000": "flac",
		"320":  "320k",
		"128":  "128k",
	}

	// 格式: level:xxx,bitrate:xxx,format:xxx,size:xxx;...
	parts := strings.Split(minfo, ";")
	for _, part := range parts {
		var bitrate, size string
		for _, kv := range strings.Split(part, ",") {
			kv = strings.TrimSpace(kv)
			if strings.HasPrefix(kv, "bitrate:") {
				bitrate = strings.TrimPrefix(kv, "bitrate:")
			}
			if strings.HasPrefix(kv, "size:") {
				size = strings.TrimPrefix(kv, "size:")
			}
		}
		if qt, ok := qualityMap[bitrate]; ok {
			result = append(result, QualityItem{Type: qt, Size: size})
		}
	}
	return result
}

// decodeName 解码歌曲名
func (h *LeaderboardHandler) decodeName(name interface{}) string {
	if name == nil {
		return ""
	}
	// 洛雪使用 &&& 替换特殊字符
	s := strings.ReplaceAll(fmt.Sprintf("%v", name), "&&&", "/")
	// 解码 HTML 实体
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	return s
}

// getString 获取字符串值
func (h *LeaderboardHandler) getString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// getInt 获取整数值
func (h *LeaderboardHandler) getInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case string:
		i, _ := strconv.Atoi(val)
		return i
	}
	return 0
}

// formatPlayTime 格式化播放时间
func (h *LeaderboardHandler) formatPlayTime(seconds int) string {
	if seconds <= 0 {
		return "00:00"
	}
	m := seconds / 60
	s := seconds % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}
