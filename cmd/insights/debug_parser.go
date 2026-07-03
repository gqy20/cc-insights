package main

import (
	"regexp"
)

var mcpPattern = regexp.MustCompile(`mcp__(\w+)__(\w+)`)
