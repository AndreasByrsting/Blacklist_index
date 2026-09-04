package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// MaxImageSize 单张图片最大字节数（浏览器端已压缩，通常远低于此）。
const MaxImageSize = 5 << 20 // 5MB

// ErrImageTypeInvalid 图片格式不支持（魔数校验失败）。
var ErrImageTypeInvalid = errors.New("仅支持 JPG、PNG、WebP 格式的图片")

var allowedImageExts = map[string]bool{
	"jpg":  true,
	"jpeg": true,
	"png":  true,
	"webp": true,
}

// ImageService 负责上传图片的磁盘存储：两层哈希目录 + SHA-256 文件名。
type ImageService struct {
	uploadDir string
}

// NewImageService 构造图片存储服务，上传目录为 dataDir/upload。
func NewImageService(dataDir string) *ImageService {
	return &ImageService{uploadDir: filepath.Join(dataDir, "upload")}
}

// DetectImageExt 通过魔数识别图片真实格式，返回规范化扩展名（jpg/png/webp）。
func DetectImageExt(data []byte) (string, error) {
	switch {
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "jpg", nil
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}):
		return "png", nil
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "webp", nil
	default:
		return "", ErrImageTypeInvalid
	}
}

// IsImageExtAllowed 判断扩展名是否在允许列表内（图片服务读取时校验）。
func IsImageExtAllowed(ext string) bool {
	return allowedImageExts[ext]
}

// Save 将图片写入两层哈希目录，返回文件哈希与扩展名。
func (s *ImageService) Save(data []byte) (string, string, error) {
	ext, err := DetectImageExt(data)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	dir := filepath.Join(s.uploadDir, hash[:2], hash[2:4])
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(filepath.Join(dir, hash+"."+ext), data, 0o644); err != nil {
		return "", "", err
	}
	return hash, ext, nil
}

// Path 返回某哈希图片在磁盘上的绝对路径。
func (s *ImageService) Path(hash, ext string) string {
	return filepath.Join(s.uploadDir, hash[:2], hash[2:4], hash+"."+ext)
}

// StoredImage 表示磁盘上现存的一张证据图片。
type StoredImage struct {
	Hash string
	Ext  string
	Path string
}

// ListStoredImages 遍历上传目录，返回所有合法的图片文件（64 位十六进制哈希 + 允许扩展名）。
func (s *ImageService) ListStoredImages() ([]StoredImage, error) {
	out := make([]StoredImage, 0)
	if _, err := os.Stat(s.uploadDir); os.IsNotExist(err) {
		return out, nil
	}
	err := filepath.WalkDir(s.uploadDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(d.Name())), ".")
		if !IsImageExtAllowed(ext) {
			return nil
		}
		name := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		if len(name) != 64 || !isHexHash64(name) {
			return nil
		}
		out = append(out, StoredImage{Hash: name, Ext: ext, Path: p})
		return nil
	})
	return out, err
}

// isHexHash64 判断字符串是否为 64 位小写十六进制（SHA-256 文件名）。
func isHexHash64(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}