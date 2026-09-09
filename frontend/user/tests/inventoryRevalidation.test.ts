import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'

import {
  INVENTORY_REVALIDATION_INTERVAL_MS,
  startInventoryRevalidation,
} from '../src/utils/inventoryRevalidation.ts'

const flush = async () => {
  await Promise.resolve()
  await Promise.resolve()
}

test('keeps visible stock within the realtime refresh budget', () => {
  assert.equal(INVENTORY_REVALIDATION_INTERVAL_MS, 3_000)
})

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

test('background inventory revalidation does not replace products with loading skeletons', () => {
  const source = (relativePath: string) =>
    readFileSync(new URL(relativePath, import.meta.url), 'utf8')

  const productList = source('../src/composables/useProductList.ts')
  const home = source('../src/views/Home.vue')
  const products = source('../src/views/Products.vue')
  const vaultHome = source('../src/templates/vault/Home.vue')

  assert.match(productList, /const loadProducts = async \(showLoading = true\)/)
  assert.match(productList, /if \(showLoading\) loading\.value = true/)
  assert.match(productList, /if \(showLoading\) loading\.value = false/)
  assert.match(home, /await listLoadProducts\(false\)/)
  assert.match(products, /startInventoryRevalidation\(\(\) => loadProducts\(false\)\)/)
  assert.match(vaultHome, /await listLoadProducts\(false\)/)
  assert.match(vaultHome, /await loadProducts\(false\)/)
})
