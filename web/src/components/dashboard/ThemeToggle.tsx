import { useEffect, useState } from 'react'
import { useTheme } from 'next-themes'
import { Moon, Sun } from 'lucide-react'
import { Button } from '@/components/ui/button'

// §3.7 暗色切换。next-themes 已在 main.tsx 接入；图表色走 CSS 变量，暗色自动适配。
export function ThemeToggle() {
  const { resolvedTheme, setTheme } = useTheme()
  const [mounted, setMounted] = useState(false)
  useEffect(() => setMounted(true), [])

  if (!mounted) return <Button variant="outline" size="icon" className="opacity-0" />

  const isDark = resolvedTheme === 'dark'
  return (
    <Button
      variant="outline"
      size="icon"
      onClick={() => setTheme(isDark ? 'light' : 'dark')}
      aria-label="切换主题"
      title={isDark ? '切换到亮色' : '切换到暗色'}
    >
      {isDark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
    </Button>
  )
}
