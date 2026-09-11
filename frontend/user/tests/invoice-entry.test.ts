import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const read = (path: string) => readFile(new URL(path, import.meta.url), 'utf8')

test('every storefront navigation exposes the invoice entry', async () => {
  const nav = await read('../src/composables/useNavConfig.ts')
  assert.match(nav, /key:\s*'invoice',\s*path:\s*'\/invoice'/)

  const zh = JSON.parse(await read('../src/i18n/locales/zh-CN.json'))
  const tw = JSON.parse(await read('../src/i18n/locales/zh-TW.json'))
  const en = JSON.parse(await read('../src/i18n/locales/en-US.json'))
  assert.equal(zh.nav.invoice, '开发票')
  assert.equal(tw.nav.invoice, '開發票')
  assert.equal(en.nav.invoice, 'Invoice')
})

test('invoice form asks for the receiving email only once', async () => {
  const invoice = await read('../src/views/Invoice.vue')
  assert.match(invoice, /发票接收邮箱/)
  assert.doesNotMatch(invoice, /再次确认邮箱|confirm_email|两次输入的发票接收邮箱/)
})

test('invoice page unwraps the shared API response envelope', async () => {
  const invoice = await read('../src/views/Invoice.vue')
  assert.match(invoice, /invoiceAPI\.get\(requestNo\)\)\.data\.data/)
  assert.match(invoice, /result\.value = response\.data\.data/)
})

test('invoice status copy does not expose internal finance or Feishu workflow', async () => {
  const invoice = await read('../src/views/Invoice.vue')
  assert.match(invoice, /系统正在处理/)
  assert.match(invoice, /发票开好后会自动发送到你填写的邮箱/)
  assert.doesNotMatch(invoice, /财务待办|飞书待办|财务将在飞书/)
})

test('authenticated invoice applications can choose fee-free wallet payment', async () => {
  const invoice = await read('../src/views/Invoice.vue')
  assert.match(invoice, /value="wallet"/)
  assert.match(invoice, /钱包/)
  assert.match(invoice, /免新增支付通道手续费/)
  assert.match(invoice, /value="alipay"/)
  assert.match(invoice, /payment_method/)
})

test('invoice form is ordinary-only and previews the tax-inclusive total before payment', async () => {
  const invoice = await read('../src/views/Invoice.vue')
  assert.doesNotMatch(invoice, /专用发票|value="special"|公司地址|开户银行/)
  assert.match(invoice, /开票费用预览/)
	assert.match(invoice, /v-model="form\.invoice_amount"/)
	assert.match(invoice, /invoice_amount:\s*String\(form\.invoice_amount/)
	assert.match(invoice, /发票金额（价税合计）/)
	assert.match(invoice, /请填写发票上显示的价税合计金额/)
	assert.match(invoice, /开票服务费（3%）/)
	assert.match(invoice, /支付通道手续费/)
	assert.match(invoice, /本次应付/)
})

test('invoice payment channels are radio choices below the receiving email with a generic submit label', async () => {
  const invoice = await read('../src/views/Invoice.vue')
  const email = invoice.indexOf('发票接收邮箱')
  const payment = invoice.indexOf('<fieldset')
  assert.ok(email >= 0 && payment > email)
  assert.match(invoice, /type="radio"[^>]+value="alipay"/)
  assert.match(invoice, /type="radio"[^>]+value="wallet"/)
  assert.match(invoice, /确认资料并支付/)
  assert.doesNotMatch(invoice, /确认资料并使用钱包支付/)
})

test('offline-transfer invoice applications do not require a platform order number', async () => {
  const invoice = await read('../src/views/Invoice.vue')
  const api = await read('../src/api/order.ts')
  assert.match(invoice, /线下转账可不填/)
  assert.doesNotMatch(invoice, /v-model="form\.order_no" required/)
  assert.doesNotMatch(invoice, />订单实际结算金额</)
  assert.match(invoice, /invoiceAPI\.previewManual/)
  assert.match(invoice, /invoiceAPI\.createManual/)
  assert.match(api, /previewManual/)
  assert.match(api, /createManual/)
})
