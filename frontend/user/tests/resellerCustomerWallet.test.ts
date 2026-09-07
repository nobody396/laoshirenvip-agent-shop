import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

test('reseller console exposes tenant customer wallets and authenticated SKU pricing', async () => {
  const [view, priceView, api, productApi, router, layout, wallet] = await Promise.all([
    readFile(new URL('../src/views/reseller/ResellerCustomers.vue', import.meta.url), 'utf8'),
    readFile(new URL('../src/views/reseller/ResellerCustomerPrices.vue', import.meta.url), 'utf8'),
    readFile(new URL('../src/api/reseller.ts', import.meta.url), 'utf8'),
    readFile(new URL('../src/api/product.ts', import.meta.url), 'utf8'),
    readFile(new URL('../src/router/index.ts', import.meta.url), 'utf8'),
    readFile(new URL('../src/views/reseller/ResellerConsoleLayout.vue', import.meta.url), 'utf8'),
    readFile(new URL('../src/views/personal/WalletPanel.vue', import.meta.url), 'utf8'),
  ])

  assert.match(api, /customers:\s*\(/)
  assert.match(api, /topUpCustomerWallet/)
  assert.match(api, /customerWalletTransactions/)
  assert.match(api, /customerPrices/)
  assert.match(api, /setCustomerPrice/)
  assert.match(api, /resetCustomerPrice/)
  assert.match(router, /path:\s*'customers'/)
  assert.match(router, /path:\s*'customers\/:id\/prices'/)
  assert.match(layout, /resellerConsole\.nav\.customers/)
  assert.match(view, /request_id/)
  assert.match(view, /wallet_balance/)
  assert.match(view, /specialPrices/)
  assert.match(priceView, /base_price_amount/)
  assert.match(priceView, /effective_price_amount/)
  assert.match(priceView, /setCustomerPrice/)
  assert.match(priceView, /resetCustomerPrice/)
  assert.match(productApi, /authenticated\(\).*userApi\.get\('\/products'/s)
  assert.match(wallet, /walletScope === 'reseller'/)
  assert.match(wallet, /resellerWalletManagedHint/)
})
