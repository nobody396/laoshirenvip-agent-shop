export const INVENTORY_REVALIDATION_INTERVAL_MS = 30_000

type InventoryRevalidationOptions = {
  windowTarget?: EventTarget
  documentTarget?: EventTarget
  isVisible?: () => boolean
  intervalMs?: number
  setIntervalFn?: (callback: () => void, intervalMs: number) => unknown
  clearIntervalFn?: (id: unknown) => void
}

export function startInventoryRevalidation(
  refresh: () => void | Promise<void>,
  options: InventoryRevalidationOptions = {},
) {
  const windowTarget = options.windowTarget ?? window
  const documentTarget = options.documentTarget ?? document
  const isVisible = options.isVisible ?? (() => document.visibilityState !== 'hidden')
  const intervalMs = options.intervalMs ?? INVENTORY_REVALIDATION_INTERVAL_MS
  const setIntervalFn = options.setIntervalFn ?? ((callback, delay) => window.setInterval(callback, delay))
  const clearIntervalFn = options.clearIntervalFn ?? ((id) => window.clearInterval(id as number))
  let refreshing = false
  let stopped = false

  const run = async () => {
    if (stopped || refreshing || !isVisible()) return
    refreshing = true
    try {
      await refresh()
    } finally {
      refreshing = false
    }
  }
  const onFocus = () => void run()
  const onVisibilityChange = () => void run()
  const timer = setIntervalFn(() => void run(), intervalMs)

  windowTarget.addEventListener('focus', onFocus)
  documentTarget.addEventListener('visibilitychange', onVisibilityChange)

  return () => {
    stopped = true
    clearIntervalFn(timer)
    windowTarget.removeEventListener('focus', onFocus)
    documentTarget.removeEventListener('visibilitychange', onVisibilityChange)
  }
}
