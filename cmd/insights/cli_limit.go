package main

import (
	"sort"
	"strings"
)

type nameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func limitSlice[T any](items []T, limit int) []T {
	if len(items) <= limit {
		return append([]T(nil), items...)
	}
	return append([]T(nil), items[:limit]...)
}

func topNameCounts(values map[string]int, limit int) []nameCount {
	items := make([]nameCount, 0, len(values))
	for name, count := range values {
		if strings.TrimSpace(name) == "" || count <= 0 {
			continue
		}
		items = append(items, nameCount{Name: name, Count: count})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Name < items[j].Name
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}
