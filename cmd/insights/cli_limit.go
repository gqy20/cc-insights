package main

func limitSlice[T any](items []T, limit int) []T {
	if len(items) <= limit {
		return append([]T(nil), items...)
	}
	return append([]T(nil), items[:limit]...)
}
