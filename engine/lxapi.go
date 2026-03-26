//go:build wasip1

// Package engine 封装 goja JS 运行时，用于执行洛雪音源脚本。
package engine

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/dop251/goja"
	"github.com/mimusic-org/plugin/pkg/go-plugin-http/http"
)

// LxAPI 管理 globalThis.lx 的注入
type LxAPI struct {
	vm            *goja.Runtime
	eventHandlers map[string]goja.Callable // on() 注册的处理器
	sourceConfig  *SourceConfig            // inited 事件接收的配置
}

// NewLxAPI 创建新的 LxAPI 实例
func NewLxAPI(vm *goja.Runtime) *LxAPI {
	return &LxAPI{
		vm:            vm,
		eventHandlers: make(map[string]goja.Callable),
	}
}

// GetSourceConfig 获取从 inited 事件解析的音源配置
func (l *LxAPI) GetSourceConfig() *SourceConfig {
	return l.sourceConfig
}

// CallEventHandler 调用注册的事件处理器
func (l *LxAPI) CallEventHandler(event string, args ...interface{}) (goja.Value, error) {
	handler, ok := l.eventHandlers[event]
	if !ok {
		return nil, fmt.Errorf("no handler registered for event: %s", event)
	}

	// 转换参数为 goja.Value
	jsArgs := make([]goja.Value, len(args))
	for i, arg := range args {
		jsArgs[i] = l.vm.ToValue(arg)
	}

	return handler(goja.Undefined(), jsArgs...)
}

// InjectLxAPI 注入 globalThis.lx 对象到 goja 运行时
func (l *LxAPI) InjectLxAPI() error {
	// 创建 lx 对象
	lxObj := l.vm.NewObject()

	// lx.version
	if err := lxObj.Set("version", "2.1.0"); err != nil {
		return fmt.Errorf("set lx.version: %w", err)
	}

	// lx.env
	if err := lxObj.Set("env", "desktop"); err != nil {
		return fmt.Errorf("set lx.env: %w", err)
	}

	// lx.EVENT_NAMES
	eventNames := l.vm.NewObject()
	_ = eventNames.Set("inited", "inited")
	_ = eventNames.Set("request", "request")
	if err := lxObj.Set("EVENT_NAMES", eventNames); err != nil {
		return fmt.Errorf("set lx.EVENT_NAMES: %w", err)
	}

	// lx.on(event, handler) - 注册事件处理器
	if err := lxObj.Set("on", l.createOnFunc()); err != nil {
		return fmt.Errorf("set lx.on: %w", err)
	}

	// lx.send(event, data) - 发送事件
	if err := lxObj.Set("send", l.createSendFunc()); err != nil {
		return fmt.Errorf("set lx.send: %w", err)
	}

	// lx.request(url, options, callback) - HTTP 请求
	if err := lxObj.Set("request", l.createRequestFunc()); err != nil {
		return fmt.Errorf("set lx.request: %w", err)
	}

	// lx.utils
	utils := l.vm.NewObject()
	if err := l.setupUtils(utils); err != nil {
		return fmt.Errorf("setup lx.utils: %w", err)
	}
	if err := lxObj.Set("utils", utils); err != nil {
		return fmt.Errorf("set lx.utils: %w", err)
	}

	// 注入到 globalThis
	global := l.vm.GlobalObject()
	if err := global.Set("lx", lxObj); err != nil {
		return fmt.Errorf("set globalThis.lx: %w", err)
	}

	return nil
}

// createOnFunc 创建 lx.on 函数
func (l *LxAPI) createOnFunc() func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			slog.Warn("lx.on: 参数不足")
			return goja.Undefined()
		}

		event := call.Argument(0).String()
		handlerVal := call.Argument(1)

		handler, ok := goja.AssertFunction(handlerVal)
		if !ok {
			slog.Warn("lx.on: 第二个参数不是函数", "event", event)
			return goja.Undefined()
		}

		l.eventHandlers[event] = handler
		slog.Debug("lx.on: 注册事件处理器", "event", event)

		return goja.Undefined()
	}
}

// createSendFunc 创建 lx.send 函数
func (l *LxAPI) createSendFunc() func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			slog.Warn("lx.send: 参数不足")
			return goja.Undefined()
		}

		event := call.Argument(0).String()
		data := call.Argument(1).Export()

		slog.Debug("lx.send: 收到事件", "event", event)

		switch event {
		case "inited":
			l.handleInitedEvent(data)
		default:
			slog.Debug("lx.send: 未知事件", "event", event)
		}

		return goja.Undefined()
	}
}

// handleInitedEvent 处理 inited 事件
func (l *LxAPI) handleInitedEvent(data interface{}) {
	slog.Debug("处理 inited 事件", "data", data)

	// 将 data 转换为 JSON 再解析为 SourceConfig
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		slog.Warn("inited 事件: JSON 序列化失败", "error", err)
		return
	}

	var config SourceConfig
	if err := json.Unmarshal(jsonBytes, &config); err != nil {
		slog.Warn("inited 事件: JSON 反序列化失败", "error", err)
		return
	}

	l.sourceConfig = &config
	slog.Info("音源配置已加载", "sources", len(config.Sources))
}

// createRequestFunc 创建 lx.request 函数
func (l *LxAPI) createRequestFunc() func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 3 {
			slog.Warn("lx.request: 参数不足")
			return goja.Undefined()
		}

		url := call.Argument(0).String()
		optionsVal := call.Argument(1).Export()
		callbackVal := call.Argument(2)

		callback, ok := goja.AssertFunction(callbackVal)
		if !ok {
			slog.Warn("lx.request: 第三个参数不是函数")
			return goja.Undefined()
		}

		// 解析 options
		options := l.parseHTTPOptions(optionsVal)

		slog.Debug("lx.request: 发起请求", "url", url, "method", options.Method)

		// 执行 HTTP 请求
		resp, err := l.doHTTPRequest(url, options)
		if err != nil {
			// 调用回调，传递错误
			errObj := l.vm.NewObject()
			_ = errObj.Set("message", err.Error())
			_, _ = callback(goja.Undefined(), l.vm.ToValue(errObj), goja.Null(), goja.Null())
			return goja.Undefined()
		}

		// 调用回调，传递响应
		respObj := l.vm.NewObject()
		_ = respObj.Set("statusCode", resp.StatusCode)
		_ = respObj.Set("headers", resp.Headers)
		_ = respObj.Set("body", resp.Body)

		_, _ = callback(goja.Undefined(), goja.Null(), l.vm.ToValue(respObj), l.vm.ToValue(resp.Body))
		return goja.Undefined()
	}
}

// parseHTTPOptions 解析 HTTP 选项
func (l *LxAPI) parseHTTPOptions(val interface{}) HTTPOptions {
	options := HTTPOptions{
		Method:  "GET",
		Headers: make(map[string]string),
		Timeout: 30000,
	}

	if val == nil {
		return options
	}

	optMap, ok := val.(map[string]interface{})
	if !ok {
		return options
	}

	if method, ok := optMap["method"].(string); ok {
		options.Method = strings.ToUpper(method)
	}

	if headers, ok := optMap["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				options.Headers[k] = vs
			}
		}
	}

	if body, ok := optMap["body"].(string); ok {
		options.Body = body
	}

	if timeout, ok := optMap["timeout"].(float64); ok {
		options.Timeout = int(timeout)
	}

	return options
}

// doHTTPRequest 执行 HTTP 请求
func (l *LxAPI) doHTTPRequest(url string, options HTTPOptions) (*HTTPResponse, error) {
	var bodyReader io.Reader
	if options.Body != "" {
		bodyReader = strings.NewReader(options.Body)
	}

	req, err := http.NewRequest(options.Method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 设置请求头
	for k, v := range options.Headers {
		req.Header.Set(k, v)
	}

	// 设置默认 User-Agent
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "lx-music-desktop/2.1.0")
	}

	// 执行请求
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// 构建响应头 map
	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return &HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

// setupUtils 设置 lx.utils 对象
func (l *LxAPI) setupUtils(utils *goja.Object) error {
	// lx.utils.buffer
	buffer := l.vm.NewObject()
	if err := buffer.Set("from", l.createBufferFromFunc()); err != nil {
		return err
	}
	if err := utils.Set("buffer", buffer); err != nil {
		return err
	}

	// lx.utils.crypto
	crypto := l.vm.NewObject()
	if err := crypto.Set("md5", l.createMD5Func()); err != nil {
		return err
	}
	if err := utils.Set("crypto", crypto); err != nil {
		return err
	}

	return nil
}

// createBufferFromFunc 创建 lx.utils.buffer.from 函数
func (l *LxAPI) createBufferFromFunc() func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}

		data := call.Argument(0).String()
		encoding := "utf8"
		if len(call.Arguments) > 1 {
			encoding = call.Argument(1).String()
		}

		// 创建一个简单的 buffer 对象
		bufObj := l.vm.NewObject()
		_ = bufObj.Set("data", data)
		_ = bufObj.Set("encoding", encoding)
		_ = bufObj.Set("toString", func(call goja.FunctionCall) goja.Value {
			return l.vm.ToValue(data)
		})

		return bufObj
	}
}

// createMD5Func 创建 lx.utils.crypto.md5 函数
func (l *LxAPI) createMD5Func() func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}

		str := call.Argument(0).String()
		hash := md5.Sum([]byte(str))
		return l.vm.ToValue(hex.EncodeToString(hash[:]))
	}
}
