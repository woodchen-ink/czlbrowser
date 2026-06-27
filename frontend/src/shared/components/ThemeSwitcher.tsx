import { Check } from 'lucide-react'
import clsx from 'clsx'
import { useTheme, themeConfigs, ThemeType } from '../theme'

interface ThemeSwitcherProps {
  className?: string
}

const themePreview: Record<ThemeType, { bg: string; sidebar: string; accent: string }> = {
  light: {
    bg: 'var(--color-preview-light-bg)',
    sidebar: 'var(--color-preview-light-sidebar)',
    accent: 'var(--color-preview-light-accent)',
  },
  dark: {
    bg: 'var(--color-preview-dark-bg)',
    sidebar: 'var(--color-preview-dark-sidebar)',
    accent: 'var(--color-preview-dark-accent)',
  },
}

export function ThemeSwitcher({ className }: ThemeSwitcherProps) {
  const { theme, setTheme } = useTheme()

  return (
    <div className={clsx('space-y-4', className)}>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {themeConfigs.map((config) => {
          const isActive = theme === config.id
          const preview = themePreview[config.id]

          return (
            <button
              key={config.id}
              onClick={() => setTheme(config.id)}
              className={clsx(
                'group relative flex items-center gap-4 rounded-2xl border p-4 text-left transition-all duration-200',
                isActive
                  ? 'border-[var(--color-accent)] bg-[var(--color-accent-soft)]'
                  : 'border-[var(--color-border-default)] bg-[var(--color-bg-surface)] hover:border-[var(--color-border-strong)] hover:bg-[var(--color-bg-subtle)]'
              )}
              title={config.description}
            >
              <div
                className="w-28 shrink-0 overflow-hidden rounded-xl border border-[var(--color-preview-line)]"
                style={{ backgroundColor: preview.bg }}
              >
                <div className="flex aspect-[5/4]">
                  <div className="flex w-1/3 flex-col gap-1 px-2 py-2" style={{ backgroundColor: preview.sidebar }}>
                    <div className="h-1 rounded-full" style={{ backgroundColor: preview.accent }} />
                    <div className="h-1 rounded-full bg-[var(--color-preview-line)]" />
                    <div className="h-1 rounded-full bg-[var(--color-preview-line)]" />
                  </div>
                  <div className="flex-1 px-2 py-2">
                    <div className="mb-2 h-1.5 w-2/3 rounded-full bg-[var(--color-preview-line)]" />
                    <div className="grid grid-cols-2 gap-1">
                      <div className="h-6 rounded-md" style={{ backgroundColor: preview.sidebar }} />
                      <div className="h-6 rounded-md" style={{ backgroundColor: preview.sidebar }} />
                    </div>
                    <div className="mt-2 h-1 rounded-full" style={{ backgroundColor: preview.accent, opacity: 0.75 }} />
                  </div>
                </div>
              </div>

              <div className="min-w-0 flex-1">
                <div className="flex items-center justify-between gap-3">
                  <span
                    className={clsx(
                      'text-sm font-semibold transition-colors',
                      isActive ? 'text-[var(--color-text-primary)]' : 'text-[var(--color-text-secondary)]'
                    )}
                  >
                    {config.name.replace('主题', '')}
                  </span>
                  {isActive && (
                    <div className="flex h-5 w-5 items-center justify-center rounded-full bg-[var(--color-accent)] shadow-sm">
                      <Check className="h-3 w-3 text-[var(--color-text-inverse)]" />
                    </div>
                  )}
                </div>
                <p className="mt-1 text-xs leading-5 text-[var(--color-text-muted)]">{config.description}</p>
              </div>

              {isActive && (
                <div className="absolute inset-x-4 bottom-0 h-px bg-[var(--color-accent)]" />
              )}
            </button>
          )
        })}
      </div>
    </div>
  )
}
