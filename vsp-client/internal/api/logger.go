package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type LogEntry struct {
	Time    string `json:"time"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

type Logger struct {
	logs      []LogEntry
	mu        sync.RWMutex
	maxLogs   int
	logFile   *os.File
	enableFile bool
}

func NewLogger() *Logger {
	return &Logger{
		maxLogs: 1000,
		logs:    []LogEntry{},
	}
}

func NewLoggerWithFile(dir string) (*Logger, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	filename := filepath.Join(dir, fmt.Sprintf("vsp-client-%s.log", time.Now().Format("2006-01-02")))
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &Logger{
		maxLogs:   1000,
		logs:      []LogEntry{},
		logFile:   f,
		enableFile: true,
	}, nil
}

func (l *Logger) Log(t string, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Time:    time.Now().Format("2006-01-02 15:04:05"),
		Type:    t,
		Message: msg,
	}

	l.logs = append(l.logs, entry)
	if len(l.logs) > l.maxLogs {
		l.logs = l.logs[len(l.logs)-l.maxLogs:]
	}

	if l.enableFile && l.logFile != nil {
		line, _ := json.Marshal(entry)
		l.logFile.Write(append(line, '\n'))
	}
}

func (l *Logger) GetLogs(limit int) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if limit > len(l.logs) {
		limit = len(l.logs)
	}

	start := len(l.logs) - limit
	if start < 0 {
		start = 0
	}

	result := make([]LogEntry, limit)
	copy(result, l.logs[start:])

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

func (l *Logger) Close() {
	if l.logFile != nil {
		l.logFile.Close()
	}
}

type DataLog struct {
	Time      string `json:"time"`
	Tunnel    string `json:"tunnel"`
	Direction string `json:"direction"`
	Size      int    `json:"size"`
	Data      string `json:"data,omitempty"`
}

type DataLogger struct {
	logs      []DataLog
	mu        sync.RWMutex
	maxLogs   int
	logFile   *os.File
	enableFile bool
}

func NewDataLogger() *DataLogger {
	return &DataLogger{
		maxLogs: 5000,
		logs:    []DataLog{},
	}
}

func (l *DataLogger) Log(tunnel, direction string, data []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := DataLog{
		Time:      time.Now().Format("2006-01-02 15:04:05"),
		Tunnel:    tunnel,
		Direction: direction,
		Size:      len(data),
		Data:      string(data),
	}

	l.logs = append(l.logs, entry)
	if len(l.logs) > l.maxLogs {
		l.logs = l.logs[len(l.logs)-l.maxLogs:]
	}
}

func (l *DataLogger) GetLogs(limit int) []DataLog {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if limit > len(l.logs) {
		limit = len(l.logs)
	}

	start := len(l.logs) - limit
	if start < 0 {
		start = 0
	}

	result := make([]DataLog, limit)
	copy(result, l.logs[start:])

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}
