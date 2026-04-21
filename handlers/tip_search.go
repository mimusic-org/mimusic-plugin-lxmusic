//go:build wasip1

package handlers

import (
	"crypto/aes"
	"crypto/cipher"
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

// TipSearchHandler 搜索联想处理器
type TipSearchHandler struct{}

// NewTipSearchHandler 创建搜索联想处理器
func NewTipSearchHandler() *TipSearchHandler {
	return &TipSearchHandler{}
}

func (h *TipSearchHandler) HandleTipSearch(req *http.Request) (*plugin.RouterResponse, error) {
	source := req.URL.Query().Get("source")
	if source == "" {
		return plugin.ErrorResponse(http.StatusBadRequest, "缺少 source 参数"), nil
	}

	name := req.URL.Query().Get("name")
	if name == "" {
		return plugin.ErrorResponse(http.StatusBadRequest, "缺少 name 参数"), nil
	}

	var list []string
	var err error

	switch source {
	case "kw":
		list, err = h.getKuwoTipSearch(name)
	case "kg":
		list, err = h.getKgTipSearch(name)
	case "tx":
		list, err = h.getTxTipSearch(name)
	case "wy":
		list, err = h.getWyTipSearch(name)
	case "mg":
		list, err = h.getMgTipSearch(name)
	default:
		return plugin.ErrorResponse(http.StatusBadRequest, "暂不支持该平台: "+source), nil
	}

	if err != nil {
		slog.Error("获取搜索联想失败", "source", source, "name", name, "error", err)
		return plugin.ErrorResponse(http.StatusInternalServerError, "获取搜索联想失败: "+err.Error()), nil
	}

	if isTVRequest(req) {
		body, _ := json.Marshal(list)
		return &plugin.RouterResponse{
			StatusCode: http.StatusOK,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       body,
		}, nil
	}

	response := map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": list,
	}
	body, _ := json.Marshal(response)
	return &plugin.RouterResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}, nil
}

func (h *TipSearchHandler) getKuwoTipSearch(name string) ([]string, error) {
	urlStr := "https://tips.kuwo.cn/t.s?corp=kuwo&newver=3&p2p=1&notrace=0&c=mbox&w=" +
		url.QueryEscape(name) + "&encoding=utf8&rformat=json"

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
		WORDITEMS []struct {
			RELWORD string `json:"RELWORD"`
		} `json:"WORDITEMS"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if len(result.WORDITEMS) == 0 {
		return []string{}, nil
	}

	list := make([]string, 0, len(result.WORDITEMS))
	for _, item := range result.WORDITEMS {
		list = append(list, item.RELWORD)
	}
	return list, nil
}

func (h *TipSearchHandler) getKgTipSearch(name string) ([]string, error) {
	urlStr := fmt.Sprintf("https://searchtip.kugou.com/getSearchTip?MusicTipCount=10&keyword=%s", url.QueryEscape(name))

	req, err := pluginhttp.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://www.kugou.com/")

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

	var result []struct {
		RecordDatas []struct {
			HintInfo string `json:"HintInfo"`
		} `json:"RecordDatas"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return []string{}, nil
	}

	list := make([]string, 0)
	for _, item := range result[0].RecordDatas {
		list = append(list, item.HintInfo)
	}
	return list, nil
}

func (h *TipSearchHandler) getTxTipSearch(name string) ([]string, error) {
	urlStr := fmt.Sprintf("https://c.y.qq.com/splcloud/fcgi-bin/smartbox_new.fcg?is_xml=0&format=json&key=%s&loginUin=0&hostUin=0&format=json&inCharset=utf8&outCharset=utf-8&notice=0&platform=yqq&needNewCode=0",
		url.QueryEscape(name))

	req, err := pluginhttp.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://y.qq.com/portal/player.html")

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
			Song struct {
				Itemlist []struct {
					Name   string `json:"name"`
					Singer string `json:"singer"`
				} `json:"itemlist"`
			} `json:"song"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("code: %d", result.Code)
	}

	list := make([]string, 0, len(result.Data.Song.Itemlist))
	for _, item := range result.Data.Song.Itemlist {
		list = append(list, fmt.Sprintf("%s - %s", item.Name, item.Singer))
	}
	return list, nil
}

func (h *TipSearchHandler) getWyTipSearch(name string) ([]string, error) {
	params, encSecKey, err := h.weapiEncrypt(map[string]interface{}{
		"s": name,
	})
	if err != nil {
		return nil, fmt.Errorf("weapi加密失败: %w", err)
	}

	formData := url.Values{}
	formData.Set("params", params)
	formData.Set("encSecKey", encSecKey)

	req, err := pluginhttp.NewRequest("POST", "https://music.163.com/weapi/search/suggest/web", strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "https://music.163.com")
	req.Header.Set("Origin", "https://music.163.com")

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
		Result struct {
			Songs []struct {
				Name    string `json:"name"`
				Artists []struct {
					Name string `json:"name"`
				} `json:"artists"`
			} `json:"songs"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Code != 200 {
		return nil, fmt.Errorf("code: %d", result.Code)
	}

	list := make([]string, 0, len(result.Result.Songs))
	for _, song := range result.Result.Songs {
		var singerNames []string
		for _, artist := range song.Artists {
			singerNames = append(singerNames, artist.Name)
		}
		list = append(list, fmt.Sprintf("%s - %s", song.Name, strings.Join(singerNames, "、")))
	}
	return list, nil
}

func (h *TipSearchHandler) getMgTipSearch(name string) ([]string, error) {
	urlStr := fmt.Sprintf("https://app.u.nf.migu.cn/pc/resource/content/tone_search_suggest/v1.0?text=%s", url.QueryEscape(name))

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
		SongList []struct {
			SongName string `json:"songName"`
		} `json:"songList"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	list := make([]string, 0, len(result.SongList))
	for _, item := range result.SongList {
		list = append(list, item.SongName)
	}
	return list, nil
}

func (h *TipSearchHandler) weapiEncrypt(object interface{}) (params string, encSecKey string, err error) {
	text, err := json.Marshal(object)
	if err != nil {
		return "", "", fmt.Errorf("marshal: %w", err)
	}

	secKey := randomStringTip(16)

	encText, err := aesCBCEncryptTip(string(text), "0CoJUm6Qyw8W8jud", "0102030405060708")
	if err != nil {
		return "", "", fmt.Errorf("first aes: %w", err)
	}

	params, err = aesCBCEncryptTip(encText, secKey, "0102030405060708")
	if err != nil {
		return "", "", fmt.Errorf("second aes: %w", err)
	}

	encSecKey = rsaEncryptTip(secKey)

	return params, encSecKey, nil
}

func randomStringTip(size int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, size)
	for i := range b {
		b[i] = letters[i%len(letters)]
	}
	return string(b)
}

func pkcs7PadTip(data []byte, blockSize int) []byte {
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

func aesCBCEncryptTip(text, key, iv string) (string, error) {
	keyBytes := []byte(key)
	ivBytes := []byte(iv)
	srcBytes := []byte(text)

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	srcBytes = pkcs7PadTip(srcBytes, block.BlockSize())
	blockMode := cipher.NewCBCEncrypter(block, ivBytes)
	crypted := make([]byte, len(srcBytes))
	blockMode.CryptBlocks(crypted, srcBytes)

	return base64.StdEncoding.EncodeToString(crypted), nil
}

func rsaEncryptTip(text string) string {
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
