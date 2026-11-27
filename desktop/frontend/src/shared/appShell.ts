/**
 * 客户端壳内的文字选择: 界面默认不可选, 只有内容区可选 (样式在 index.css 的 html[data-app-shell] 下)。
 * 只在 Wails 壳里生效, 用普通浏览器直接打开 Vite 开发页时保持浏览器默认。
 */

/** 可整块全选的内容: 最近一次点在这些区域里时, Ctrl/⌘+A 只选中这一块。 */
const SELECT_BLOCK = '[data-select-block], pre'

function inWailsShell(): boolean {
  const w = window as unknown as Record<string, unknown>
  return 'WailsInvoke' in w || 'runtime' in w || 'go' in w
}

/** 焦点在可输入的地方时 Ctrl/⌘+A 照常全选输入内容。 */
function isEditable(el: Element | null): boolean {
  if (!(el instanceof HTMLElement)) return false
  if (el.isContentEditable || el instanceof HTMLTextAreaElement) return true
  if (el instanceof HTMLInputElement) {
    return !['checkbox', 'radio', 'button', 'submit', 'reset', 'range', 'color', 'file', 'image'].includes(el.type)
  }
  return false
}

export function installAppShellSelection() {
  if (!inWailsShell()) return
  document.documentElement.dataset.appShell = 'wails'

  let lastBlock: Element | null = null
  document.addEventListener(
    'pointerdown',
    (e) => {
      lastBlock = e.target instanceof Element ? e.target.closest(SELECT_BLOCK) : null
    },
    true,
  )

  document.addEventListener(
    'keydown',
    (e) => {
      if (e.isComposing || !(e.ctrlKey || e.metaKey) || e.altKey || e.shiftKey || e.key.toLowerCase() !== 'a') return
      if (isEditable(document.activeElement)) return
      e.preventDefault()
      const sel = window.getSelection()
      if (!sel) return
      sel.removeAllRanges()
      if (lastBlock?.isConnected) {
        const range = document.createRange()
        range.selectNodeContents(lastBlock)
        sel.addRange(range)
      }
    },
    true,
  )
}
