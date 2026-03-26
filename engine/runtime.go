//go:build wasip1

// Package engine 封装 goja JS 运行时，用于执行洛雪音源脚本。
package engine

import (
	"fmt"
	"log/slog"

	"github.com/dop251/goja"
)

// Runtime 管理 JS 运行时
type Runtime struct{}

// NewRuntime 创建新的 Runtime 实例
func NewRuntime() *Runtime {
	return &Runtime{}
}

// LoadSource 加载并执行音源脚本，返回解析到的 SourceConfig
func (r *Runtime) LoadSource(script string) (*SourceConfig, error) {
	vm := goja.New()

	// 创建并注入 lx API
	lxAPI := NewLxAPI(vm)
	if err := lxAPI.InjectLxAPI(); err != nil {
		return nil, fmt.Errorf("inject lx API: %w", err)
	}

	// 设置 console.log 等
	r.setupConsole(vm)

	// 执行脚本
	_, err := vm.RunString(script)
	if err != nil {
		return nil, fmt.Errorf("execute script: %w", err)
	}

	// 获取 SourceConfig
	config := lxAPI.GetSourceConfig()
	if config == nil {
		return nil, fmt.Errorf("script did not call send('inited', ...)")
	}

	return config, nil
}

// CallRequest 调用 request 事件处理器
// 支持同步返回值和 Promise（async function）返回值
// source: 来源平台标识（如 "kw", "tx"）
// action: 动作类型（如 "musicUrl", "search"）
// info: 请求信息
func (r *Runtime) CallRequest(script string, source string, action string, info map[string]interface{}) (interface{}, error) {
	vm := goja.New()

	// 创建并注入 lx API
	lxAPI := NewLxAPI(vm)
	if err := lxAPI.InjectLxAPI(); err != nil {
		return nil, fmt.Errorf("inject lx API: %w", err)
	}

	// 设置 console.log 等
	r.setupConsole(vm)

	// 执行脚本
	_, err := vm.RunString(script)
	if err != nil {
		return nil, fmt.Errorf("execute script: %w", err)
	}

	// 构建请求参数
	payload := map[string]interface{}{
		"source": source,
		"action": action,
		"info":   info,
	}

	// 调用 request 事件处理器
	result, err := lxAPI.CallEventHandler("request", payload)
	if err != nil {
		return nil, fmt.Errorf("call request handler: %w", err)
	}

	// 处理返回值
	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		return nil, nil
	}

	// 检查是否为 Promise
	if p, ok := result.Export().(*goja.Promise); ok {
		return r.resolvePromise(vm, p)
	}

	return result.Export(), nil
}

// resolvePromise 等待 goja Promise 解析完成
// 通过反复执行 vm.RunString("") 来 flush microtask 队列
// 由于 lx.request() 是同步的，Promise 链不涉及真正的异步等待
func (r *Runtime) resolvePromise(vm *goja.Runtime, p *goja.Promise) (interface{}, error) {
	const maxIterations = 1000
	for i := 0; i < maxIterations; i++ {
		switch p.State() {
		case goja.PromiseStateFulfilled:
			result := p.Result()
			if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
				return nil, nil
			}
			// 如果结果仍然是 Promise，递归解析
			if nestedP, ok := result.Export().(*goja.Promise); ok {
				return r.resolvePromise(vm, nestedP)
			}
			return result.Export(), nil
		case goja.PromiseStateRejected:
			result := p.Result()
			if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
				return nil, fmt.Errorf("promise rejected")
			}
			return nil, fmt.Errorf("promise rejected: %v", result.Export())
		default:
			// Promise 仍然 Pending，flush microtask 队列
			_, _ = vm.RunString("")
		}
	}
	return nil, fmt.Errorf("promise did not resolve after %d iterations", maxIterations)
}

// GetMusicUrl 获取歌曲播放 URL
func (r *Runtime) GetMusicUrl(script string, source string, musicInfo map[string]interface{}, quality string) (string, error) {
	// 构建请求信息
	info := map[string]interface{}{
		"musicInfo": musicInfo,
		"type":      quality,
	}

	// 调用 request 处理器
	result, err := r.CallRequest(script, source, "musicUrl", info)
	if err != nil {
		return "", err
	}

	// 处理返回值
	if result == nil {
		return "", fmt.Errorf("no result returned")
	}

	// 尝试获取 URL
	switch v := result.(type) {
	case string:
		return v, nil
	case map[string]interface{}:
		if url, ok := v["url"].(string); ok {
			return url, nil
		}
	}

	return "", fmt.Errorf("unexpected result type: %T", result)
}

// Search 搜索歌曲
func (r *Runtime) Search(script string, source string, keyword string, page int, limit int) (*SearchResult, error) {
	// 构建请求信息
	info := map[string]interface{}{
		"keyword": keyword,
		"page":    page,
		"limit":   limit,
	}

	// 调用 request 处理器
	result, err := r.CallRequest(script, source, "search", info)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, fmt.Errorf("no result returned")
	}

	// 解析结果
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}

	searchResult := &SearchResult{
		Source: source,
	}

	// 解析 list
	if list, ok := resultMap["list"].([]interface{}); ok {
		for _, item := range list {
			if itemMap, ok := item.(map[string]interface{}); ok {
				searchItem := SearchItem{
					Source: source,
				}
				if name, ok := itemMap["name"].(string); ok {
					searchItem.Name = name
				}
				if singer, ok := itemMap["singer"].(string); ok {
					searchItem.Singer = singer
				}
				if album, ok := itemMap["album"].(string); ok {
					searchItem.Album = album
				}
				if duration, ok := itemMap["duration"].(float64); ok {
					searchItem.Duration = int(duration)
				}
				if musicID, ok := itemMap["musicId"].(string); ok {
					searchItem.MusicID = musicID
				} else if musicID, ok := itemMap["music_id"].(string); ok {
					searchItem.MusicID = musicID
				}
				if img, ok := itemMap["img"].(string); ok {
					searchItem.Img = img
				}
				searchResult.List = append(searchResult.List, searchItem)
			}
		}
	}

	// 解析 total
	if total, ok := resultMap["total"].(float64); ok {
		searchResult.Total = int(total)
	} else {
		searchResult.Total = len(searchResult.List)
	}

	return searchResult, nil
}

// setupConsole 设置 console 对象
func (r *Runtime) setupConsole(vm *goja.Runtime) {
	console := vm.NewObject()

	logFunc := func(level string) func(goja.FunctionCall) goja.Value {
		return func(call goja.FunctionCall) goja.Value {
			args := make([]interface{}, len(call.Arguments))
			for i, arg := range call.Arguments {
				args[i] = arg.Export()
			}
			switch level {
			case "debug":
				slog.Debug("JS console", "args", args)
			case "info":
				slog.Info("JS console", "args", args)
			case "warn":
				slog.Warn("JS console", "args", args)
			case "error":
				slog.Error("JS console", "args", args)
			default:
				slog.Info("JS console", "args", args)
			}
			return goja.Undefined()
		}
	}

	_ = console.Set("log", logFunc("info"))
	_ = console.Set("debug", logFunc("debug"))
	_ = console.Set("info", logFunc("info"))
	_ = console.Set("warn", logFunc("warn"))
	_ = console.Set("error", logFunc("error"))

	_ = vm.Set("console", console)
}
