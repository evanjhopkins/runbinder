package platform

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"time"
)

type Logger struct {
	path string
	mu   sync.Mutex
}

func NewLogger(path string) *Logger {
	return &Logger{path: path}
}

func (l *Logger) Write(message string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = fmt.Fprintf(file, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"), message)
	return err
}

func Tail(path string, count int) ([]string, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	lines := make([]string, 0, count)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if len(lines) == count {
			copy(lines, lines[1:])
			lines[count-1] = scanner.Text()
		} else {
			lines = append(lines, scanner.Text())
		}
	}
	return lines, scanner.Err()
}
