import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const read = (path: string) => readFileSync(new URL(`../src/${path}`, import.meta.url), 'utf8')

test('sale-disabled catalog stays visible while every purchase entry is blocked', () => {
  const detail = read('composables/useProductDetail.ts')
  const quickBuy = read('components/ProductQuickBuy.vue')
  const cards = [
    read('components/ProductCard.vue'),
    read('components/ProductListItem.vue'),
    read('templates/vault/components/VaultProductCard.vue'),
    read('templates/vault/components/VaultProductListItem.vue'),
  ]

  assert.match(detail, /product\.value\.sale_disabled\) return false/)
  assert.match(detail, /!isSkuSaleDisabled\(sku\) && isSkuSelectable\(sku\)/)
  assert.match(quickBuy, /props\.product\.sale_disabled\) return false/)
  assert.match(quickBuy, /product\?\.sale_disabled \|\| selectedSku\?\.sale_disabled/)
  for (const card of cards) {
    assert.match(card, /saleDisabled/)
  }
})
