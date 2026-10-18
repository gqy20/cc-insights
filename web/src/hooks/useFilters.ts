import { useSyncExternalStore, useCallback } from 'react'
import type { Filters } from '@/api/types'
import { PRESETS } from '@/api/types'

const PROVIDERS = ['claude', 'codex', 'opencode', 'pi', 'all'] as const

// 全局 filter 单源：读写 window.location.search，无 react-router。
// preset 变化 → Query key 变化 → 自动重取（见 api/hooks.ts）。

function readFromURL(): Filters {
  const p = new URLSearchParams(window.location.search)
  const raw = p.get('preset')
  const preset = raw && (PRESETS as readonly string[]).includes(raw) ? raw : '30d'
  const rawProvider = p.get('provider')
  const provider = rawProvider && (PROVIDERS as readonly string[]).includes(rawProvider) ? rawProvider as Filters['provider'] : 'claude'
  return {
    preset,
    provider,
    start: p.get('start') || undefined,
    end: p.get('end') || undefined,
    project: p.get('project') || undefined,
    tool: p.get('tool') || undefined,
    model: p.get('model') || undefined,
    reason: p.get('reason') || undefined,
  }
}

let snapshot: Filters = readFromURL()
const listeners = new Set<() => void>()

function subscribe(cb: () => void) {
  listeners.add(cb)
  return () => listeners.delete(cb)
}

export function useFilters(): [Filters, (patch: Partial<Filters>) => void] {
  const filters = useSyncExternalStore(subscribe, () => snapshot, () => snapshot)

  const setFilters = useCallback((patch: Partial<Filters>) => {
    snapshot = { ...snapshot, ...patch }
    const url = new URL(window.location.href)
    for (const [k, v] of Object.entries(snapshot)) {
      if (v) url.searchParams.set(k, v)
      else url.searchParams.delete(k)
    }
    window.history.replaceState({}, '', url)
    listeners.forEach((cb) => cb())
  }, [])

  return [filters, setFilters]
}
