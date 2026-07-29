package main

import (
	"bufio"
	"errors"
	"io"
)

// maxJSONLLineBytes 单行 JSONL 的最大长度(64 MiB)。远超正常日志行,
// 仅用于防止异常超长行耗尽内存。
const maxJSONLLineBytes = 64 << 20

const jsonlReadBufferBytes = 64 << 10

type jsonlScanner struct {
	reader       *bufio.Reader
	line         []byte
	maxLineBytes int
	err          error
}

// newJSONLScanner 返回按行读取 JSONL 的 scanner。
//
// 相比流式 json.Decoder,逐行 Unmarshal 可在遇到损坏行(例如内容被
// NUL 字节污染的截断写入)时安全跳过该行并继续解析后续行;而
// json.Decoder 在遇到非法字节时既不报 EOF 也不前移读取位置,配合
// “遇错 continue”的循环会陷入 100% CPU 死循环(一条坏行即可挂死整次解析)。
// 超过上限的异常行会被完整丢弃,后续行仍可继续读取。
func newJSONLScanner(r io.Reader) *jsonlScanner {
	return newJSONLScannerWithLimit(r, maxJSONLLineBytes)
}

func newJSONLScannerWithLimit(r io.Reader, maxLineBytes int) *jsonlScanner {
	if maxLineBytes <= 0 {
		maxLineBytes = maxJSONLLineBytes
	}
	return &jsonlScanner{
		reader:       bufio.NewReaderSize(r, jsonlReadBufferBytes),
		maxLineBytes: maxLineBytes,
	}
}

func (s *jsonlScanner) Scan() bool {
	if s.err != nil {
		return false
	}

	s.line = s.line[:0]
	overLimit := false
	for {
		fragment, err := s.reader.ReadSlice('\n')
		if !overLimit {
			if len(fragment) > s.maxLineBytes-len(s.line) {
				overLimit = true
				s.line = s.line[:0]
			} else {
				s.line = append(s.line, fragment...)
			}
		}

		switch {
		case err == nil:
			if overLimit {
				overLimit = false
				s.line = s.line[:0]
				continue
			}
			s.trimLineEnding()
			return true
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF):
			if overLimit || len(s.line) == 0 {
				return false
			}
			s.trimLineEnding()
			return true
		default:
			s.err = err
			return false
		}
	}
}

func (s *jsonlScanner) Bytes() []byte {
	return s.line
}

func (s *jsonlScanner) Err() error {
	return s.err
}

func (s *jsonlScanner) trimLineEnding() {
	if n := len(s.line); n > 0 && s.line[n-1] == '\n' {
		s.line = s.line[:n-1]
	}
	if n := len(s.line); n > 0 && s.line[n-1] == '\r' {
		s.line = s.line[:n-1]
	}
}
