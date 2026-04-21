//go:build wasip1

package handlers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"strings"

	"github.com/mimusic-org/plugin/api/plugin"
	pluginhttp "github.com/mimusic-org/plugin/pkg/go-plugin-http/http"
)

type HotSearchHandler struct{}

func NewHotSearchHandler() *HotSearchHandler {
	return &HotSearchHandler{}
}

func (h *HotSearchHandler) HandleHotSearch(req *http.Request) (*plugin.RouterResponse, error) {
	source := req.URL.Query().Get("source")
	if source == "" {
		return plugin.ErrorResponse(http.StatusBadRequest, "缺少 source 参数"), nil
	}

	var list []string
	var err error

	switch source {
	case "kw":
		list, err = h.getKuwoHotSearch()
	case "kg":
		list, err = h.getKgHotSearch()
	case "tx":
		list, err = h.getTxHotSearch()
	case "wy":
		list, err = h.getWyHotSearch()
	case "mg":
		list, err = h.getMgHotSearch()
	default:
		return plugin.ErrorResponse(http.StatusBadRequest, "暂不支持该平台: "+source), nil
	}

	if err != nil {
		slog.Error("获取热搜失败", "source", source, "error", err)
		return plugin.ErrorResponse(http.StatusInternalServerError, "获取热搜失败: "+err.Error()), nil
	}

	if isTVRequest(req) {
		response := map[string]interface{}{
			"source": source,
			"list":   list,
		}
		body, _ := json.Marshal(response)
		return &plugin.RouterResponse{
			StatusCode: http.StatusOK,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       body,
		}, nil
	}

	response := map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": map[string]interface{}{
			"source": source,
			"list":   list,
		},
	}
	body, _ := json.Marshal(response)
	return &plugin.RouterResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}, nil
}

func (h *HotSearchHandler) getKuwoHotSearch() ([]string, error) {
	urlStr := "http://hotword.kuwo.cn/hotword.s?prod=kwplayer_ar_9.3.0.1&corp=kuwo&newver=2&vipver=9.3.0.1&source=kwplayer_ar_9.3.0.1_40.apk&p2p=1&notrace=0&uid=0&plat=kwplayer_ar&rformat=json&encoding=utf8&tabid=1"

	resp, err := pluginhttp.Get(urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Status   string `json:"status"`
		Tagvalue []struct {
			Key string `json:"key"`
		} `json:"tagvalue"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Status != "ok" {
		return nil, fmt.Errorf("status: %s", result.Status)
	}

	list := make([]string, 0, len(result.Tagvalue))
	for _, item := range result.Tagvalue {
		list = append(list, item.Key)
	}
	return list, nil
}

func (h *HotSearchHandler) getKgHotSearch() ([]string, error) {
	urlStr := "http://gateway.kugou.com/api/v3/search/hot_tab?signature=ee44edb9d7155821412d220bcaf509dd&appid=1005&clientver=10026&plat=0"

	req, err := pluginhttp.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("dfid", "1ssiv93oVqMp27cirf2CvoF1")
	req.Header.Set("mid", "156798703528610303473757548878786007104")
	req.Header.Set("clienttime", "1584257267")
	req.Header.Set("x-router", "msearch.kugou.com")
	req.Header.Set("user-agent", "Android9-AndroidPhone-10020-130-0-searchrecommendprotocol-wifi")
	req.Header.Set("kg-rc", "1")

	resp, err := pluginhttp.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Errcode int `json:"errcode"`
		Data    struct {
			List []struct {
				Keywords []struct {
					Keyword string `json:"keyword"`
				} `json:"keywords"`
			} `json:"list"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Errcode != 0 {
		return nil, fmt.Errorf("errcode: %d", result.Errcode)
	}

	list := make([]string, 0)
	for _, item := range result.Data.List {
		for _, kw := range item.Keywords {
			list = append(list, h.decodeName(kw.Keyword))
		}
	}
	return list, nil
}

func (h *HotSearchHandler) getTxHotSearch() ([]string, error) {
	postData := map[string]interface{}{
		"comm": map[string]interface{}{
			"ct":    "19",
			"cv":    "1803",
			"guid":  "0",
			"patch": "118",
			"uin":   "0",
			"wid":   "0",
		},
		"hotkey": map[string]interface{}{
			"method": "GetHotkeyForQQMusicPC",
			"module": "tencent_musicsoso_hotkey.HotkeyService",
			"param": map[string]interface{}{
				"search_id": "",
				"uin":       0,
			},
		},
	}

	postBody, _ := json.Marshal(postData)
	req, err := pluginhttp.NewRequest("POST", "https://u.y.qq.com/cgi-bin/musicu.fcg", strings.NewReader(string(postBody)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://y.qq.com/portal/player.html")
	req.Header.Set("Content-Type", "application/json")

	resp, err := pluginhttp.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code   int `json:"code"`
		Hotkey struct {
			Data struct {
				VecHotkey []struct {
					Query string `json:"query"`
				} `json:"vec_hotkey"`
			} `json:"data"`
		} `json:"hotkey"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("code: %d", result.Code)
	}

	list := make([]string, 0, len(result.Hotkey.Data.VecHotkey))
	for _, item := range result.Hotkey.Data.VecHotkey {
		list = append(list, item.Query)
	}
	return list, nil
}

func (h *HotSearchHandler) getWyHotSearch() ([]string, error) {
	params, err := h.eapiEncrypt("/api/search/chart/detail", map[string]interface{}{
		"id": "HOT_SEARCH_SONG#@#",
	})
	if err != nil {
		return nil, fmt.Errorf("eapi加密失败: %w", err)
	}

	formData := url.Values{}
	formData.Set("params", params)

	req, err := pluginhttp.NewRequest("POST", "http://interface.music.163.com/eapi/batch", strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "https://music.163.com")

	resp, err := pluginhttp.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			ItemList []struct {
				SearchWord string `json:"searchWord"`
			} `json:"itemList"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Code != 200 {
		return nil, fmt.Errorf("code: %d", result.Code)
	}

	list := make([]string, 0, len(result.Data.ItemList))
	for _, item := range result.Data.ItemList {
		list = append(list, item.SearchWord)
	}
	return list, nil
}

func (h *HotSearchHandler) getMgHotSearch() ([]string, error) {
	resp, err := pluginhttp.Get("http://jadeite.migu.cn:7090/music_search/v3/search/hotword")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code string `json:"code"`
		Data struct {
			Hotwords []struct {
				HotwordList []struct {
					Word string `json:"word"`
				} `json:"hotwordList"`
			} `json:"hotwords"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Code != "000000" {
		return nil, fmt.Errorf("code: %s", result.Code)
	}

	if len(result.Data.Hotwords) == 0 {
		return []string{}, nil
	}

	list := make([]string, 0)
	for _, item := range result.Data.Hotwords[0].HotwordList {
		list = append(list, item.Word)
	}
	return list, nil
}

func (h *HotSearchHandler) weapiEncrypt(object interface{}) (params string, encSecKey string, err error) {
	text, err := json.Marshal(object)
	if err != nil {
		return "", "", fmt.Errorf("marshal: %w", err)
	}

	secKey := h.randomString(16)

	encText, err := h.aesCBCEncrypt(string(text), "0CoJUm6Qyw8W8jud", "0102030405060708")
	if err != nil {
		return "", "", fmt.Errorf("first aes: %w", err)
	}

	params, err = h.aesCBCEncrypt(encText, secKey, "0102030405060708")
	if err != nil {
		return "", "", fmt.Errorf("second aes: %w", err)
	}

	encSecKey = h.rsaEncrypt(secKey)

	return params, encSecKey, nil
}

func (h *HotSearchHandler) randomString(size int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, size)
	for i := range b {
		b[i] = letters[i%len(letters)]
	}
	return string(b)
}

func (h *HotSearchHandler) pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	if padding == 0 {
		padding = blockSize
	}
	padtext := make([]byte, padding)
	for i := range padtext {
		padtext[i] = byte(padding)
	}
	return append(data, padtext...)
}

func (h *HotSearchHandler) aesCBCEncrypt(text, key, iv string) (string, error) {
	keyBytes := []byte(key)
	ivBytes := []byte(iv)
	srcBytes := []byte(text)

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	srcBytes = h.pkcs7Pad(srcBytes, block.BlockSize())
	blockMode := cipher.NewCBCEncrypter(block, ivBytes)
	crypted := make([]byte, len(srcBytes))
	blockMode.CryptBlocks(crypted, srcBytes)

	return base64.StdEncoding.EncodeToString(crypted), nil
}

func (h *HotSearchHandler) rsaEncrypt(text string) string {
	pubKey := "010001"
	modulus := "00e0b509f6259df8642dbc35662901477df22677ec152b5ff68ace615bb7b725152b3ab17a876aea8a5aa76d2e417629ec4ee341f56135fccf695280104e0312ecbda92557c93870114af6c9d05c4f7f0c3685b7a46bee255932575cce10b424d813cfe4875d3e82047b97ddef52741d546b8e289dc6935b3ece0462db0a22b8e7"

	runes := []rune(text)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	text = string(runes)

	hexText := hex.EncodeToString([]byte(text))

	biText := new(big.Int)
	biText.SetString(hexText, 16)

	biPub := new(big.Int)
	biPub.SetString(pubKey, 16)

	biMod := new(big.Int)
	biMod.SetString(modulus, 16)

	biRet := new(big.Int).Exp(biText, biPub, biMod)

	return fmt.Sprintf("%0256x", biRet)
}

func (h *HotSearchHandler) decodeName(name interface{}) string {
	if name == nil {
		return ""
	}
	s := strings.ReplaceAll(fmt.Sprintf("%v", name), "&&&", "/")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	return s
}

var eapiKey = []byte("e82ckenh8dichen8")

func (h *HotSearchHandler) eapiEncrypt(urlStr string, object interface{}) (string, error) {
	text, err := json.Marshal(object)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	message := fmt.Sprintf("nobody%suse%smd5forencrypt", urlStr, string(text))
	digest := fmt.Sprintf("%x", md5.Sum([]byte(message)))

	data := fmt.Sprintf("%s-36cd479b6b5-%s-36cd479b6b5-%s", urlStr, string(text), digest)

	block, err := aes.NewCipher(eapiKey)
	if err != nil {
		return "", err
	}

	plaintext := pkcs7PadEapi([]byte(data), aes.BlockSize)
	ciphertext := make([]byte, len(plaintext))
	for i := 0; i < len(plaintext); i += aes.BlockSize {
		block.Encrypt(ciphertext[i:i+aes.BlockSize], plaintext[i:i+aes.BlockSize])
	}

	return strings.ToUpper(hex.EncodeToString(ciphertext)), nil
}

func pkcs7PadEapi(data []byte, blockSize int) []byte {
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
