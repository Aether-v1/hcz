// Package localfile 提供工单附件的私有读取能力（不暴露公开 URL）。
package localfile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Store 从本地 uploads 根目录按 objectKey 读取文件。
type Store struct {
	root string
}

// New 创建本地文件读取器。root 为 uploads 根目录。
func New(root string) *Store {
	if root == "" {
		root = "uploads"
	}
	return &Store{root: root}
}

// ReadObject 按 objectKey 读取文件内容。objectKey 为相对 uploads 根目录的路径。
// 做路径穿越防护：解析后必须仍在 root 内。
func (s *Store) ReadObject(objectKey string) (io.ReadCloser, error) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return nil, fmt.Errorf("object key is empty")
	}
	// 去掉可能的 /uploads/ 前缀，统一成相对路径。
	objectKey = strings.TrimPrefix(objectKey, "/uploads/")
	objectKey = strings.TrimPrefix(objectKey, "uploads/")

	clean := filepath.Clean(objectKey)
	if strings.HasPrefix(clean, "..") {
		return nil, fmt.Errorf("invalid object key")
	}
	full := filepath.Join(s.root, clean)
	// 防路径穿越：解析后必须仍位于 root 内。
	absRoot, err := filepath.Abs(s.root)
	if err != nil {
		return nil, err
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(absFull, absRoot+string(os.PathSeparator)) && absFull != absRoot {
		return nil, fmt.Errorf("invalid object key")
	}
	return os.Open(absFull)
}
