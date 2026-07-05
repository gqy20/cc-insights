package main

import (
	"bufio"
	"os"
)

// maxJSONLLineBytes 单行 JSONL 的最大长度(64 MiB)。远超正常日志行,
// 仅用于防止异常超长行耗尽内存。
const maxJSONLLineBytes = 64 << 20

// newJSONLScanner 返回按行读取 JSONL 文件的 scanner。
//
// 相比流式 json.Decoder,逐行 Unmarshal 可在遇到损坏行(例如内容被
// NUL 字节污染的截断写入)时安全跳过该行并继续解析后续行;而
// json.Decoder 在遇到非法字节时既不报 EOF 也不前移读取位置,配合
// “遇错 continue”的循环会陷入 100% CPU 死循环(一条坏行即可挂死整次解析)。
func newJSONLScanner(f *os.File) *bufio.Scanner {
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), maxJSONLLineBytes)
	return scanner
}
