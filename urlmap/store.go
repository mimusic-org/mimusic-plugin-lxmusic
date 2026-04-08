//go:build wasip1

package urlmap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Store URL 映射存储
type Store struct {
	dataDir  string
	filePath string
	mappings map[string]*MusicUrlMapping // hash → mapping
}

// NewStore 创建映射存储，从 JSON 文件加载已有数据
func NewStore(dataDir string) (*Store, error) {
	s := &Store{
		dataDir:  dataDir,
		filePath: filepath.Join(dataDir, "urlmap.json"),
		mappings: make(map[string]*MusicUrlMapping),
	}

	// 从文件加载已有数据
	if err := s.load(); err != nil {
		// 文件不存在不是错误
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("load urlmap: %w", err)
		}
	}

	return s, nil
}

// PutBatchItem 批量写入的单条输入
type PutBatchItem struct {
	SongInfo map[string]interface{}
	Quality  string
	Platform string
}

// Put 存入映射，返回 hash 值
// hash 生成规则：将 songInfo + quality 序列化为 JSON（key 排序），取 SHA256 前 16 位 hex
func (s *Store) Put(songInfo map[string]interface{}, quality, platform string) (string, error) {
	hash := s.generateHash(songInfo, quality)

	s.mappings[hash] = &MusicUrlMapping{
		SongInfo:  songInfo,
		Quality:   quality,
		Platform:  platform,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	if err := s.save(); err != nil {
		// 回滚
		delete(s.mappings, hash)
		return "", fmt.Errorf("save urlmap: %w", err)
	}

	return hash, nil
}

// PutBatch 批量存入映射，所有条目写入内存后只执行一次持久化。
// 返回与输入顺序一致的 hash 列表。如果持久化失败，所有条目会被回滚。
func (s *Store) PutBatch(items []PutBatchItem) ([]string, error) {
	if len(items) == 0 {
		return nil, nil
	}

	now := time.Now().Format(time.RFC3339)
	hashes := make([]string, 0, len(items))

	for _, item := range items {
		hash := s.generateHash(item.SongInfo, item.Quality)
		s.mappings[hash] = &MusicUrlMapping{
			SongInfo:  item.SongInfo,
			Quality:   item.Quality,
			Platform:  item.Platform,
			CreatedAt: now,
		}
		hashes = append(hashes, hash)
	}

	if err := s.save(); err != nil {
		// 回滚：移除本批次写入的所有条目
		for _, hash := range hashes {
			delete(s.mappings, hash)
		}
		return nil, fmt.Errorf("save urlmap batch: %w", err)
	}

	return hashes, nil
}

// Get 根据 hash 获取映射
func (s *Store) Get(hash string) (*MusicUrlMapping, bool) {
	mapping, exists := s.mappings[hash]
	return mapping, exists
}

// Delete 删除映射
func (s *Store) Delete(hash string) error {
	mapping, exists := s.mappings[hash]
	if !exists {
		return fmt.Errorf("mapping not found: %s", hash)
	}

	delete(s.mappings, hash)

	if err := s.save(); err != nil {
		// 回滚
		s.mappings[hash] = mapping
		return fmt.Errorf("save urlmap: %w", err)
	}

	return nil
}

// generateHash 生成唯一哈希
// 将 songInfo 和 quality 组合后序列化（key 排序确保确定性），取 SHA256 前 16 位 hex
func (s *Store) generateHash(songInfo map[string]interface{}, quality string) string {
	// 构建待序列化的对象
	data := map[string]interface{}{
		"songInfo": sortMapKeys(songInfo),
		"quality":  quality,
	}

	// JSON 序列化（Go 的 json.Marshal 默认按 key 排序）
	jsonBytes, _ := json.Marshal(data)

	// SHA256 哈希
	hash := sha256.Sum256(jsonBytes)

	// 取前 16 位 hex（8 字节 = 16 个 hex 字符）
	return hex.EncodeToString(hash[:8])
}

// sortMapKeys 递归排序 map 的 key（确保序列化确定性）
func sortMapKeys(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := m[k]
		// 递归处理嵌套 map
		if nested, ok := v.(map[string]interface{}); ok {
			result[k] = sortMapKeys(nested)
		} else {
			result[k] = v
		}
	}
	return result
}

// save 持久化到文件
func (s *Store) save() error {
	// 确保目录存在
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	index := &urlMapIndex{
		Version:  "1.0",
		Mappings: s.mappings,
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal urlmap: %w", err)
	}

	return os.WriteFile(s.filePath, data, 0644)
}

// load 从文件加载
func (s *Store) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var index urlMapIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return fmt.Errorf("unmarshal urlmap: %w", err)
	}

	if index.Mappings != nil {
		s.mappings = index.Mappings
	}

	return nil
}
