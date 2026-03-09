// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

const defaultLogTailChunkSize int64 = 4096

// ReadDaemonLogTail returns the last maxLines lines from the active local daemon log file.
func (s *Session) ReadDaemonLogTail(maxLines int) ([]string, error) {
	if s == nil {
		return nil, fmt.Errorf("runtime session is nil")
	}
	if s.target.Kind != TargetKindDaemon {
		return nil, fmt.Errorf("daemon log tail is only available for local daemon targets")
	}
	if strings.TrimSpace(s.target.LogFile) == "" {
		return nil, fmt.Errorf("daemon log file is unknown")
	}
	return TailFileLines(s.target.LogFile, maxLines)
}

// TailFileLines returns the last maxLines lines from path without reading the whole file when possible.
func TailFileLines(path string, maxLines int) ([]string, error) {
	if maxLines <= 0 {
		return nil, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() == 0 {
		return nil, nil
	}

	var (
		offset = info.Size()
		buf    []byte
	)
	for offset > 0 {
		chunkSize := defaultLogTailChunkSize
		if offset < chunkSize {
			chunkSize = offset
		}
		offset -= chunkSize
		chunk := make([]byte, chunkSize)
		if _, err := file.ReadAt(chunk, offset); err != nil && err != io.EOF {
			return nil, err
		}
		buf = append(chunk, buf...)
		if bytes.Count(buf, []byte{'\n'}) > maxLines {
			break
		}
	}

	text := strings.ReplaceAll(string(buf), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, nil
}
