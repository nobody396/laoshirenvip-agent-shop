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

test('continues to resolve fully localized SKU values', () => {
  assert.equal(
    formatSkuSpecValues({ 'zh-CN': '250点数额度', 'zh-TW': '250點數額度', 'en-US': '250 Credits' }, 'en-US'),
    '250 Credits',
  )
})
