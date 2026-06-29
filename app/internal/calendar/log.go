package calendar

import (
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) appendLog(file, text string) {
	if s == nil {
		return
	}
	_ = os.MkdirAll(s.logDir, 0755)
	output, _ := os.OpenFile(filepath.Join(s.logDir, file), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if output == nil {
		return
	}
	_, _ = output.WriteString(text)
	_ = output.Close()
}
func (s *Service) trimLog(file string, maxLines, keepLines int) {
	if s == nil {
		return
	}
	path := filepath.Join(s.logDir, file)
	body, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.SplitAfter(string(body), "\n")
	if len(lines) > maxLines {
		_ = os.WriteFile(path, []byte(strings.Join(lines[len(lines)-keepLines:], "")), 0644)
	}
}
