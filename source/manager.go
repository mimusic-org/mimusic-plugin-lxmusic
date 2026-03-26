//go:build wasip1
// +build wasip1

package source

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"
)

// Manager 管理已导入的音源
type Manager struct {
	sources   map[string]*SourceInfo // ID -> SourceInfo
	idCounter int                    // ID 计数器（用于生成唯一 ID）
}

// NewManager 创建一个新的音源管理器
func NewManager() *Manager {
	return &Manager{
		sources:   make(map[string]*SourceInfo),
		idCounter: 0,
	}
}

// ImportFromJS 导入单个 JS 文件
// filename: 原始文件名
// content: JS 文件内容
func (m *Manager) ImportFromJS(filename string, content []byte) (*SourceInfo, error) {
	// 1. 解析元数据
	metadata, err := ParseMetadata(content)
	if err != nil {
		return nil, fmt.Errorf("parse metadata: %w", err)
	}

	// 2. 如果没有 @name，从文件名推断
	name := metadata.Name
	if name == "" {
		name = InferNameFromFilename(filename)
	}

	// 3. 生成唯一 ID
	id := m.generateID(name)

	// 4. 创建 SourceInfo
	info := &SourceInfo{
		ID:          id,
		Name:        name,
		Version:     metadata.Version,
		Description: metadata.Description,
		Author:      metadata.Author,
		Filename:    filename,
		Script:      string(content),
		ImportedAt:  time.Now().Format(time.RFC3339),
	}

	// 5. 存入 map
	m.sources[id] = info

	slog.Info("音源导入成功", "id", id, "name", name, "filename", filename)
	return info, nil
}

// ImportFromZIP 从 ZIP 文件导入音源
// content: ZIP 文件内容
// 返回所有成功导入的 SourceInfo 列表
func (m *Manager) ImportFromZIP(content []byte) ([]*SourceInfo, error) {
	// 使用 bytes.NewReader 创建 zip.Reader
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	var imported []*SourceInfo

	// 遍历 ZIP 中的所有文件
	for _, file := range reader.File {
		// 跳过目录
		if file.FileInfo().IsDir() {
			continue
		}

		// 只处理 .js 文件
		if !strings.HasSuffix(strings.ToLower(file.Name), ".js") {
			slog.Debug("跳过非 JS 文件", "filename", file.Name)
			continue
		}

		// 读取文件内容
		fileContent, err := m.readZipFile(file)
		if err != nil {
			slog.Warn("读取 ZIP 文件失败", "filename", file.Name, "error", err)
			continue
		}

		// 导入 JS 文件
		info, err := m.ImportFromJS(filepath.Base(file.Name), fileContent)
		if err != nil {
			slog.Warn("导入音源失败", "filename", file.Name, "error", err)
			continue
		}

		imported = append(imported, info)
	}

	slog.Info("ZIP 导入完成", "total", len(imported))
	return imported, nil
}

// readZipFile 读取 ZIP 中的单个文件内容
func (m *Manager) readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open file in zip: %w", err)
	}
	defer rc.Close()

	content, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read file in zip: %w", err)
	}

	return content, nil
}

// ListSources 返回所有已导入的音源
func (m *Manager) ListSources() []*SourceInfo {
	sources := make([]*SourceInfo, 0, len(m.sources))
	for _, info := range m.sources {
		sources = append(sources, info)
	}
	return sources
}

// GetSource 按 ID 获取音源
func (m *Manager) GetSource(id string) *SourceInfo {
	return m.sources[id]
}

// GetSourceScript 获取音源的 JS 源码
func (m *Manager) GetSourceScript(id string) (string, error) {
	info := m.sources[id]
	if info == nil {
		return "", fmt.Errorf("source not found: %s", id)
	}
	return info.Script, nil
}

// DeleteSource 删除音源
func (m *Manager) DeleteSource(id string) error {
	if _, exists := m.sources[id]; !exists {
		return fmt.Errorf("source not found: %s", id)
	}
	delete(m.sources, id)
	slog.Info("音源已删除", "id", id)
	return nil
}

// Close 清理管理器
func (m *Manager) Close() {
	m.sources = make(map[string]*SourceInfo)
	m.idCounter = 0
	slog.Info("音源管理器已关闭")
}

// generateID 生成唯一 ID
// 使用 name 的 slug 形式，如果重复则添加计数器后缀
func (m *Manager) generateID(name string) string {
	// 生成基础 slug
	slug := m.toSlug(name)
	if slug == "" {
		slug = "source"
	}

	// 检查是否已存在
	id := slug
	counter := 1
	for {
		if _, exists := m.sources[id]; !exists {
			break
		}
		counter++
		id = fmt.Sprintf("%s_%d", slug, counter)
	}

	return id
}

// toSlug 将名称转换为 slug 形式
// 例如: "My Source" -> "my-source", "网易云" -> "网易云"
func (m *Manager) toSlug(name string) string {
	// 转小写
	slug := strings.ToLower(name)
	// 替换空格为横线
	slug = strings.ReplaceAll(slug, " ", "-")
	// 移除特殊字符（保留字母、数字、横线、下划线、中文）
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r >= 0x4e00 {
			result.WriteRune(r)
		}
	}
	return result.String()
}
