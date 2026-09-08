import assert from 'node:assert/strict'
import test from 'node:test'

import { startInventoryRevalidation } from '../src/utils/inventoryRevalidation.ts'

const flush = async () => {
  await Promise.resolve()
  await Promise.resolve()
}

test('revalidates visible inventory on focus, visibility restore, and interval', async () => {
  const windowTarget = new EventTarget()
  const documentTarget = new EventTarget()
  let visible = true
  let refreshes = 0
  let intervalCallback: (() => void) | undefined
  let cleared = false

  const stop = startInventoryRevalidation(
    async () => {
      refreshes += 1
    },
    {
      windowTarget,
      documentTarget,
      isVisible: () => visible,
      setIntervalFn: (callback) => {
        intervalCallback = callback
        return 7
      },
      clearIntervalFn: (id) => {
        assert.equal(id, 7)
        cleared = true
      },
    },
  )


  windowTarget.dispatchEvent(new Event('focus'))
  await flush()
  assert.equal(refreshes, 1)

  visible = false
  documentTarget.dispatchEvent(new Event('visibilitychange'))
  await flush()
  assert.equal(refreshes, 1)

  visible = true
  documentTarget.dispatchEvent(new Event('visibilitychange'))

  await flush()
  assert.equal(refreshes, 2)

  intervalCallback?.()
  await flush()
  assert.equal(refreshes, 3)

  stop()
  assert.equal(cleared, true)
  windowTarget.dispatchEvent(new Event('focus'))
  await flush()
  assert.equal(refreshes, 3)
})

test('coalesces overlapping refreshes', async () => {
  const windowTarget = new EventTarget()
  const documentTarget = new EventTarget()
  let release: (() => void) | undefined
  let refreshes = 0

  const stop = startInventoryRevalidation(
    () => {
      refreshes += 1
      return new Promise<void>((resolve) => {
        release = resolve
      })
    },
    {
      windowTarget,
      documentTarget,
      isVisible: () => true,
      setIntervalFn: () => 8,
      clearIntervalFn: () => undefined,
    },
  )

  windowTarget.dispatchEvent(new Event('focus'))
  windowTarget.dispatchEvent(new Event('focus'))
  await flush()
  assert.equal(refreshes, 1)

  release?.()
  await flush()
  windowTarget.dispatchEvent(new Event('focus'))
  await flush()
  assert.equal(refreshes, 2)
  release?.()
  stop()
})
