import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import { fulfillmentDeliveryLineURL } from '../src/utils/fulfillmentDelivery.ts'
import { fulfillmentTypeLabel } from '../src/utils/fulfillment.ts'

const read = (path: string) => readFile(new URL(path, import.meta.url), 'utf8')

test('delivery URL is clickable without accepting unsafe schemes', () => {
  assert.equal(
    fulfillmentDeliveryLineURL('充值网址: https://redeem.example/activate'),
    'https://redeem.example/activate',
  )
  assert.equal(fulfillmentDeliveryLineURL('CDK 卡密: PLUS-CDK-123'), '')
  assert.equal(fulfillmentDeliveryLineURL('充值网址: javascript:alert(1)'), '')
})

test('every order detail variant renders safe delivery URLs as links', async () => {
  for (const path of [
    '../src/views/OrderDetail.vue',
    '../src/views/GuestOrderDetail.vue',
    '../src/templates/vault/components/VaultOrderFulfillment.vue',
  ]) {
    const source = await read(path)
    assert.match(source, /fulfillmentDeliveryLineURL\(line\)/, `${path} does not link delivery URLs`)
    assert.match(source, /rel="noopener noreferrer"/, `${path} does not isolate external links`)
  }
})

test('supplier-backed fulfillment is labelled as automatic delivery', () => {
  const t = (key: string) => key.endsWith('.auto') ? '自动交付' : '人工交付'
  assert.equal(fulfillmentTypeLabel(t, 'upstream'), '自动交付')
})
