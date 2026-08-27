package log

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRotatingFileWriterBySize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	writer, err := newRotatingFileWriter(context.Background(), path, &RotateOptions{MaxSize: 1, MaxBackups: 3})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	line := strings.Repeat("a", 256*1024)
	for range 8 {
		if _, err := writer.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var backups int
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "test.log.") {
			backups++
		}
	}
	if backups == 0 {
		t.Fatal("expected rotated backup files, got none")
	}
	if backups > 3 {
		t.Fatalf("expected max 3 backups, got %d", backups)
	}
}

func TestRotatingFileWriterByAge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	writer, err := newRotatingFileWriter(context.Background(), path, &RotateOptions{MaxAge: 0, MaxBackups: 3})
	if err != nil {
		t.Fatal(err)
	}
	writer.created = time.Now().Add(-48 * time.Hour)
	writer.maxAge = 24 * time.Hour
	if _, err := writer.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var backups int
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "test.log.") {
			backups++
		}
	}
	if backups != 1 {
		t.Fatalf("expected 1 backup after age-based rotation, got %d", backups)
	}
	writer.Close()
}

func TestRotatingFileWriterBySchedule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	writer, err := newRotatingFileWriter(context.Background(), path, &RotateOptions{
		Timezone: "Asia/Shanghai",
		RotateAt: "00:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	writer.nextRotate = time.Now().Add(-1 * time.Second)
	if _, err := writer.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var backups int
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "test.log.") {
			backups++
		}
	}
	if backups != 1 {
		t.Fatalf("expected 1 backup after scheduled rotation, got %d", backups)
	}
	if writer.nextRotate.IsZero() {
		t.Fatal("expected nextRotate to be set after rotation")
	}
	if writer.nextRotate.Before(time.Now()) {
		t.Fatalf("expected nextRotate in the future, got %v", writer.nextRotate)
	}
}

func TestNextRotateTime(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC)
	next, err := nextRotateTime(location, "00:00", now)
	if err != nil {
		t.Fatal(err)
	}
	if next.Location() != location {
		t.Fatalf("expected location %v, got %v", location, next.Location())
	}
	if next.Hour() != 0 || next.Minute() != 0 {
		t.Fatalf("expected 00:00, got %v", next.Format("15:04"))
	}
	// 20:00 UTC = 北京时间 08-16 04:00，当天 00:00 已过，所以下一个是北京时间 2026-08-17 00:00
	expected := time.Date(2026, 8, 17, 0, 0, 0, 0, location)
	if !next.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, next)
	}
}

func TestNextRotateTimeBeforeCurrent(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	// 北京时间 2026-08-15 06:00，rotate_at=00:00 已经过去，下一个是 08-16 00:00
	now := time.Date(2026, 8, 15, 6, 0, 0, 0, location)
	next, err := nextRotateTime(location, "00:00", now)
	if err != nil {
		t.Fatal(err)
	}
	expected := time.Date(2026, 8, 16, 0, 0, 0, 0, location)
	if !next.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, next)
	}
}

func TestParseRotateName(t *testing.T) {
	parsed := parseRotateName("test.log.", "test.log.20260815-120000")
	if parsed.IsZero() {
		t.Fatal("expected parsed time")
	}
	if parsed.Year() != 2026 || parsed.Month() != 8 || parsed.Day() != 15 {
		t.Fatalf("unexpected parsed time: %v", parsed)
	}
}
