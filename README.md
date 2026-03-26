# mimusic-plugin-lxmusic

MiMusic 洛雪音源管理插件，用于导入和管理洛雪音源，支持歌曲搜索和批量导入到音乐库。本插件基于 [MiMusic](https://github.com/mimusic-org/mimusic) 插件系统开发。

## 功能特性

- 📦 **音源导入**：支持导入 .js 单文件或 .zip 批量导入音源
- 🔍 **元数据解析**：自动解析音源名称、版本、作者等元数据
- 🎵 **歌曲搜索**：基于导入的音源搜索网络歌曲
- 📥 **批量导入**：支持将搜索结果批量导入到 MiMusic 音乐库
- 🌐 **Web 管理界面**：提供可视化的音源管理和歌曲搜索界面

## 技术栈

- **语言**：Go（编译目标为 WASM/WASI）
- **JS 引擎**：[goja](https://github.com/dop251/goja) JavaScript 运行时
- **插件框架**：[mimusic-org/plugin](https://github.com/mimusic-org/plugin)
- **前端**：原生 HTML + CSS + JavaScript

## 项目结构

```
mimusic-plugin-lxmusic/
├── main.go                 # 插件入口，路由注册
├── handlers/
│   ├── source.go           # 音源管理 HTTP 处理器
│   └── search.go           # 搜索和导入 HTTP 处理器
├── engine/
│   ├── runtime.go          # JS 运行时管理
│   ├── lxapi.go            # 洛雪音源 API 实现
│   └── types.go            # 数据类型定义
├── source/
│   ├── manager.go          # 音源管理器
│   ├── parser.go           # 音源解析器
│   └── types.go            # 音源数据类型
├── static/
│   ├── index.html          # 插件页面
│   ├── css/                # 样式文件
│   └── js/                 # 前端逻辑
├── go.mod
├── go.sum
├── Makefile                # 构建脚本
└── README.md
```

## API 接口

| 方法 | 路径 | 描述 |
|------|------|------|
| `GET` | `/lxmusic/api/sources` | 列出所有已导入的音源 |
| `POST` | `/lxmusic/api/sources/import` | 导入音源文件（.js 或 .zip） |
| `DELETE` | `/lxmusic/api/sources?id=xxx` | 删除指定音源 |
| `GET` | `/lxmusic/api/search?keyword=xxx&source_id=xxx&page=1` | 搜索歌曲 |
| `POST` | `/lxmusic/api/songs/import` | 批量导入歌曲到音乐库 |
| `POST` | `/lxmusic/api/songs/get-url` | 获取歌曲播放 URL |

## 构建

```bash
make build
```

## 测试资源

以下是一些公开的洛雪音源资源，可用于测试：

- https://github.com/guoyue2010/lxmusic-
- https://github.com/pdone/lx-music-source

## 免责声明

- 本项目**仅供个人学习研究技术使用**，严禁任何形式的商业用途，包括但不限于售卖、牟利，不得使用本代码进行任何形式的牟利/贩卖/传播。
- 本项目数据来源原理是从各第三方音源中获取数据，因此本项目不对数据的准确性、合法性负责。使用本项目的过程中可能会产生版权数据，对于这些版权数据，本项目不拥有它们的所有权，为了避免侵权，使用者务必在 24 小时内清除使用本项目过程中所产生的版权数据。
- 本项目完全免费，仅供个人私下范围研究交流学习技术使用，并开源发布于 GitHub 面向全世界人用作对技术的学习交流，本项目不对项目内的技术可能存在违反当地法律法规的行为作保证，禁止在违反当地法律法规的情况下使用本项目，对于使用者在明知或不知当地法律法规不允许的情况下使用本项目所造成的任何违法违规行为由使用者承担，本项目不承担由此造成的任何直接、间接、特殊、偶然或结果性责任。
- 若你使用了本项目，将代表你接受以上声明。

> **注意**：本项目仅作为示例项目用于学习研究，请勿用于任何商业或违法用途。如有侵犯到任何人的合法权益，请联系作者，将在第一时间修改删除相关代码。

## License

本项目基于 [Apache License 2.0](LICENSE) 开源。
