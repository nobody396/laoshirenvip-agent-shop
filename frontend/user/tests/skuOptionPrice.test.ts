import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

test('each SKU option shows its own currency-formatted price beside its name and stock', () => {
  const view = readFileSync(new URL('../src/views/ProductDetail.vue', import.meta.url), 'utf8')
  const option = view.match(/<button\s+v-for="sku in activeSkus"[\s\S]*?<\/button>/)?.[0]
  assert.ok(option)
  assert.match(option, /formatPrice\(sku\.price_amount, siteCurrency\)/)
  assert.match(option, /skuDisplayText\(sku\)/)
  assert.match(option, /skuStockText\(sku\)/)
  assert.match(option, /shrink-0 whitespace-nowrap/)
  assert.match(option, /:disabled="!isSkuSelectable\(sku\)"/)
})
