import assert from 'node:assert/strict'
import test from 'node:test'

import { buildSkuDisplayText, formatSkuSpecValues } from '../src/utils/sku.ts'

test('localizes upstream Codex credit denominations in English', () => {
  assert.equal(
    buildSkuDisplayText({ specValues: { race: '250点数额度' }, locale: 'en-US' }),
    '250 Credits',
  )
  assert.equal(
    formatSkuSpecValues({ race: '1000点数额度' }, 'en-US'),
    '1000 Credits',
  )
})

test('keeps Codex credit denominations localized for Chinese storefronts', () => {
  assert.equal(formatSkuSpecValues({ race: '500点数额度' }, 'zh-CN'), '500点数额度')
  assert.equal(formatSkuSpecValues({ race: '500点数额度' }, 'zh-TW'), '500 點數額度')
})

test('renders a named SKU as the value without exposing the internal field name', () => {
  assert.equal(formatSkuSpecValues({ name: 'Claude Pro 1个月' }, 'zh-CN'), 'Claude Pro 1个月')
})

test('continues to resolve fully localized SKU values', () => {
  assert.equal(
    formatSkuSpecValues({ 'zh-CN': '250点数额度', 'zh-TW': '250點數額度', 'en-US': '250 Credits' }, 'en-US'),
    '250 Credits',
  )
})

test('imported SMS and iOS SKU labels follow the storefront locale after synchronization', () => {
  const rows = [
    ['Codex 美国一次性接码 1次', 'Codex US SMS Verification — One-Time', 'Codex 美國一次性接碼 1次'],
    ['Codex 美国长效接码 20–30天', 'Codex US SMS Verification — 20–30 Days', 'Codex 美國長效接碼 20–30天'],
    ['ChatGPT Pro 20X iOS 1个月', 'ChatGPT Pro 20X iOS — 1 Month', 'ChatGPT Pro 20X iOS 1個月'],
  ]
  for (const [source, english, traditional] of rows) {
    assert.equal(formatSkuSpecValues({ name: source }, 'en-US'), english)
    assert.equal(formatSkuSpecValues({ name: source }, 'en'), english)
    assert.equal(formatSkuSpecValues({ name: source }, 'zh-TW'), traditional)
    assert.equal(formatSkuSpecValues({ name: source }, 'zh-CN'), source)
  }
  assert.equal(formatSkuSpecValues({ name: 'Unrelated SKU' }, 'en-US'), 'Unrelated SKU')
})
