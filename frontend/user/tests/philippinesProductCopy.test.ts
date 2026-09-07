import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

test('all Philippine ChatGPT products keep the approved account restrictions', async () => {
  const catalog = JSON.parse(await readFile(new URL('../../../catalog/agent-products.json', import.meta.url), 'utf8'))
  const products = catalog.filter((item: any) => String(item?.title?.['zh-CN'] || '').startsWith('ChatGPT ') && String(item?.title?.['zh-CN'] || '').includes('菲律宾'))
  assert.deepEqual(products.map((item: any) => item.title['zh-CN']).sort(), [
    'ChatGPT Plus 菲律宾 1个月',
    'ChatGPT Pro 20X 菲律宾 1个月',
  ])

  for (const product of products) {
    for (const locale of ['zh-CN', 'zh-TW', 'en-US']) {
      const html = String(product.content?.[locale] || '')
      for (const required of [
        '菲律宾渠道账号限制',
        '部分 Gmail 和 iCloud 邮箱注册的账号存在充值失败概率',
        '“嘿充”等非正规渠道充值',
        '4–5 天',
        'Apple iOS',
        'Google Play',
        '2–3 天',
        'ChatGPT Team/Business 工作区',
      ]) {
        assert.ok(html.includes(required), `${product.title['zh-CN']} ${locale} missing: ${required}`)
      }
      assert.ok(html.indexOf('菲律宾渠道账号限制') < html.indexOf('充值与续费规则'))
    }
  }
})
