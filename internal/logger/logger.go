package logger

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DailyWriter 实现按日期滚动的日志文件写入。
type DailyWriter struct {
	mu   sync.Mutex
	dir  string
	date string
	file *os.File
}

// NewDailyWriter 创建日志目录并返回写入器。
func NewDailyWriter(dir string) (*DailyWriter, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &DailyWriter{dir: dir}, nil
}

func (w *DailyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	date := time.Now().Format("2006-01-02")
	if date != w.date {
		if w.file != nil {
			_ = w.file.Close()
		}
		f, err := os.OpenFile(filepath.Join(w.dir, "app-"+date+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return 0, err
		}
		w.file = f
		w.date = date
	}
	return w.file.Write(p)
}

// Close 关闭当前文件句柄。
func (w *DailyWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}
