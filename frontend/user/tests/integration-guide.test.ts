import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const read = (path: string) => readFile(new URL(path, import.meta.url), 'utf8')

test('header replaces the blog destination with the integration guide', async () => {
  const nav = await read('../src/composables/useNavConfig.ts')
  assert.match(nav, /integrationGuide:\s*\{\s*path:\s*'\/integration-guide'/)
  assert.doesNotMatch(nav, /blog:\s*\{\s*path:\s*'\/blog'/)
  assert.match(nav, /!appStore\.isResellerTenant/)
})

test('legacy blog index redirects to the integration guide', async () => {
  const router = await read('../src/router/index.ts')
  assert.match(router, /path:\s*'\/blog',[\s\S]*?redirect:\s*'\/integration-guide'/)
  assert.match(router, /path:\s*'\/integration-guide',[\s\S]*?name:\s*'integration-guide'/)
  assert.match(router, /appStore\.isResellerTenant[\s\S]*?to\.path === '\/integration-guide'/)
})

test('guide localizes the complete page and both copyable handoffs', async () => {
  const guide = await read('../src/views/IntegrationGuide.vue')
  assert.match(guide, /useI18n\(\)/)
  assert.match(guide, /integrationGuide\.guideText/)
  assert.match(guide, /integrationGuide\.ai\.prompt/)

  const zh = JSON.parse(await read('../src/i18n/locales/zh-CN.json'))
  const en = JSON.parse(await read('../src/i18n/locales/en-US.json'))

  for (const required of [
    'Dujiao OpenAPI',
    'ACG SharedStock',
    'Dujiao-Next-Signature',
    '/shared/commodity/trade',
    'request_no',
    'downstream_order_no',
    'Never ask for or output a real API Secret',
  ]) {
    assert.ok(JSON.stringify(en.integrationGuide).includes(required) || guide.includes(required), `missing English guide contract: ${required}`)
  }

  assert.equal(zh.integrationGuide.title, '发卡系统对接教程')
  assert.equal(en.integrationGuide.title, 'Card Store Integration Guide')
  assert.match(en.integrationGuide.guideText, /Laoshiren AI Partner Integration Guide/)
  assert.match(en.integrationGuide.ai.prompt, /real-funds path that remains unverified/)
})
