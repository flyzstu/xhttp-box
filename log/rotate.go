package log

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sagernet/sing/service/filemanager"
)

const rotateTimeLayout = "20060102-150405"

type rotatingFileWriter struct {
	ctx        context.Context
	file       *os.File
	path       string
	dir        string
	prefix     string
	location   *time.Location
	maxSize    int64
	maxAge     time.Duration
	maxKeep    int
	nextRotate time.Time

	size    int64
	created time.Time
}

func newRotatingFileWriter(ctx context.Context, path string, options *RotateOptions) (*rotatingFileWriter, error) {
	basePath := filemanager.BasePath(ctx, path)
	location := time.Local
	if options.Timezone != "" {
		parsedLocation, err := time.LoadLocation(options.Timezone)
		if err != nil {
			return nil, fmt.Errorf("load time zone %q: %w", options.Timezone, err)
		}
		location = parsedLocation
	}
	writer := &rotatingFileWriter{
		ctx:      ctx,
		path:     basePath,
		dir:      filepath.Dir(basePath),
		prefix:   filepath.Base(basePath) + ".",
		location: location,
		maxSize:  int64(options.MaxSize) * 1024 * 1024,
		maxAge:   time.Duration(options.MaxAge) * 24 * time.Hour,
		maxKeep:  options.MaxBackups,
	}
	if options.RotateAt != "" {
		nextRotate, err := nextRotateTime(location, options.RotateAt, time.Now())
		if err != nil {
			return nil, err
		}
		writer.nextRotate = nextRotate
	}
	file, err := filemanager.OpenFile(ctx, path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	writer.file = file
	writer.size = info.Size()
	writer.created = info.ModTime()
	return writer, nil
}

func (w *rotatingFileWriter) Write(p []byte) (n int, err error) {
	if w.needRotate(len(p)) {
		if err = w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err = w.file.Write(p)
	w.size += int64(n)
	return
}

func (w *rotatingFileWriter) Close() error {
	return w.file.Close()
}

func (w *rotatingFileWriter) needRotate(extra int) bool {
	if w.maxSize > 0 && w.size+int64(extra) > w.maxSize {
		return true
	}
	if w.maxAge > 0 && time.Since(w.created) >= w.maxAge {
		return true
	}
	if !w.nextRotate.IsZero() && !time.Now().Before(w.nextRotate) {
		return true
	}
	return false
}

func (w *rotatingFileWriter) rotate() error {
	if err := w.file.Close(); err != nil {
		return err
	}
	nowTime := time.Now().In(w.location)
	backupName := fmt.Sprintf("%s%s", w.prefix, nowTime.Format(rotateTimeLayout))
	backupPath := filepath.Join(w.dir, backupName)
	for i := 1; ; i++ {
		if _, err := filemanager.Stat(w.ctx, backupPath); errors.Is(err, os.ErrNotExist) {
			break
		}
		backupName = fmt.Sprintf("%s%s.%d", w.prefix, nowTime.Format(rotateTimeLayout), i)
		backupPath = filepath.Join(w.dir, backupName)
	}
	if err := filemanager.Rename(w.ctx, w.path, backupPath); err != nil {
		return err
	}
	file, err := filemanager.OpenFile(w.ctx, w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w.file = file
	w.size = 0
	w.created = time.Now()
	if !w.nextRotate.IsZero() {
		nextRotate, err := nextRotateTime(w.location, w.nextRotate.Format("15:04"), time.Now())
		if err == nil {
			w.nextRotate = nextRotate
		}
	}
	w.cleanup()
	return nil
}

func (w *rotatingFileWriter) cleanup() {
	if w.maxKeep <= 0 {
		return
	}
	var backups []string
	_ = filepath.WalkDir(w.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path == w.path {
			return nil
		}
		if strings.HasPrefix(d.Name(), w.prefix) {
			backups = append(backups, path)
		}
		return nil
	})
	sort.Slice(backups, func(i, j int) bool {
		leftTime := parseRotateName(w.prefix, filepath.Base(backups[i]))
		rightTime := parseRotateName(w.prefix, filepath.Base(backups[j]))
		return leftTime.Before(rightTime)
	})
	expired := len(backups) - w.maxKeep
	if expired > 0 {
		backups = backups[:expired]
	} else {
		backups = nil
	}
	for _, backupPath := range backups {
		_ = filemanager.Remove(w.ctx, backupPath)
	}
}

func parseRotateName(prefix string, name string) time.Time {
	timePart := strings.TrimPrefix(name, prefix)
	if dot := strings.IndexByte(timePart, '.'); dot >= 0 {
		timePart = timePart[:dot]
	}
	if parsed, err := time.ParseInLocation(rotateTimeLayout, timePart, time.Local); err == nil {
		return parsed
	}
	return time.Time{}
}

func nextRotateTime(location *time.Location, rotateAt string, now time.Time) (time.Time, error) {
	parsed, err := time.ParseInLocation("15:04", rotateAt, location)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse rotate_at %q: %w", rotateAt, err)
	}
	nowInLocation := now.In(location)
	next := time.Date(
		nowInLocation.Year(), nowInLocation.Month(), nowInLocation.Day(),
		parsed.Hour(), parsed.Minute(), 0, 0, location,
	)
	if !next.After(nowInLocation) {
		next = next.AddDate(0, 0, 1)
	}
	return next, nil
}
