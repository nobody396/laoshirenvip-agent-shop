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
