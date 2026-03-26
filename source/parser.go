//go:build wasip1
// +build wasip1

package source

import (
	"regexp"
	"strings"
)

// jsdocPattern 匹配 JSDoc 注释块 (/** ... */)
var jsdocPattern = regexp.MustCompile(`(?s)/\*\*[\s\S]*?\*/`)

// tagPatterns 各标签的正则表达式
var tagPatterns = map[string]*regexp.Regexp{
	"name":        regexp.MustCompile(`@name\s+(.+)`),
	"version":     regexp.MustCompile(`@version\s+(.+)`),
	"description": regexp.MustCompile(`@description\s+(.+)`),
	"author":      regexp.MustCompile(`@author\s+(.+)`),
}

// ParseMetadata 解析 JS 文件头部的 JSDoc 注释块，提取元数据
func ParseMetadata(content []byte) (*SourceMetadata, error) {
	text := string(content)
	metadata := &SourceMetadata{}

	// 查找第一个 JSDoc 注释块
	match := jsdocPattern.FindString(text)
	if match == "" {
		// 没有找到 JSDoc 注释块，返回空元数据
		return metadata, nil
	}

	// 解析各标签
	for tag, pattern := range tagPatterns {
		if m := pattern.FindStringSubmatch(match); len(m) > 1 {
			value := strings.TrimSpace(m[1])
			switch tag {
			case "name":
				metadata.Name = value
			case "version":
				metadata.Version = value
			case "description":
				metadata.Description = value
			case "author":
				metadata.Author = value
			}
		}
	}

	return metadata, nil
}

// InferNameFromFilename 从文件名推断音源名称
// 例如: "netease.js" -> "netease", "qq-music.js" -> "qq-music"
func InferNameFromFilename(filename string) string {
	// 去除扩展名
	name := strings.TrimSuffix(filename, ".js")
	// 去除路径，只保留文件名
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	return name
}
