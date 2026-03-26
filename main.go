//go:build wasip1
// +build wasip1

package main

import (
	"context"
	"embed"
	"log/slog"

	"mimusic-plugin-lxmusic/engine"
	"mimusic-plugin-lxmusic/handlers"
	"mimusic-plugin-lxmusic/source"

	"github.com/knqyf263/go-plugin/types/known/emptypb"
	"github.com/mimusic-org/plugin/api/pbplugin"
	"github.com/mimusic-org/plugin/api/plugin"
)

func main() {}

//go:embed static/*
var staticFS embed.FS

// Plugin 插件结构体
type Plugin struct {
	Version string

	staticHandler *plugin.StaticHandler
	sourceManager *source.Manager
	sourceHandler *handlers.SourceHandler
	searchHandler *handlers.SearchHandler
	jsRuntime     *engine.Runtime
}

func init() {
	plugin.RegisterPlugin(&Plugin{
		Version: "1.0.0",
	})
}

func (p *Plugin) GetPluginInfo(ctx context.Context, request *emptypb.Empty) (*pbplugin.GetPluginInfoResponse, error) {
	return &pbplugin.GetPluginInfoResponse{
		Success: true,
		Message: "成功获取插件信息",
		Info: &pbplugin.PluginInfo{
			Name:        "洛雪音源",
			Version:     p.Version,
			Description: "管理洛雪音源，搜索和导入网络歌曲",
			Author:      "MiMusic Team",
			Homepage:    "https://github.com/mimusic-org/mimusic",
			EntryPath:   "/lxmusic",
		},
	}, nil
}

func (p *Plugin) Init(ctx context.Context, request *pbplugin.InitRequest) (*emptypb.Empty, error) {
	slog.Info("正在初始化洛雪音源插件", "version", p.Version)

	// 初始化音源管理器
	p.sourceManager = source.NewManager()

	// 初始化 JS 运行时
	p.jsRuntime = engine.NewRuntime()

	// 初始化处理器
	p.sourceHandler = handlers.NewSourceHandler(p.sourceManager)
	p.searchHandler = handlers.NewSearchHandler(p.sourceManager, p.jsRuntime)

	// 获取路由管理器
	rm := plugin.GetRouterManager()

	// 初始化静态文件处理器
	p.staticHandler = plugin.NewStaticHandler(staticFS, "/lxmusic", rm, ctx)

	// 注册 API 路由（requireAuth=true）
	// 音源管理
	rm.RegisterRouter(ctx, "GET", "/lxmusic/api/sources", p.sourceHandler.HandleListSources, true)
	rm.RegisterRouter(ctx, "POST", "/lxmusic/api/sources/import", p.sourceHandler.HandleImportSource, true)
	rm.RegisterRouter(ctx, "DELETE", "/lxmusic/api/sources", p.sourceHandler.HandleDeleteSource, true)

	// 搜索和导入
	rm.RegisterRouter(ctx, "GET", "/lxmusic/api/search", p.searchHandler.HandleSearch, true)
	rm.RegisterRouter(ctx, "POST", "/lxmusic/api/songs/import", p.searchHandler.HandleImportSongs, true)
	rm.RegisterRouter(ctx, "POST", "/lxmusic/api/songs/get-url", p.searchHandler.HandleGetMusicUrl, true)

	slog.Info("洛雪音源插件路由注册完成")
	return &emptypb.Empty{}, nil
}

func (p *Plugin) Deinit(ctx context.Context, request *emptypb.Empty) (*emptypb.Empty, error) {
	slog.Info("正在反初始化洛雪音源插件")

	// 清理音源管理器
	if p.sourceManager != nil {
		p.sourceManager.Close()
	}

	return &emptypb.Empty{}, nil
}
