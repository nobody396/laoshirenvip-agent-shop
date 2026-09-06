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

test('guide contains both protocols and a safe AI handoff', async () => {
  const guide = await read('../src/views/IntegrationGuide.vue')
  for (const required of [
    'Dujiao OpenAPI',
    'ACG SharedStock',
    'Dujiao-Next-Signature',
    '/shared/commodity/trade',
    'request_no',
    'downstream_order_no',
    '不要索要或输出真实 API Secret',
  ]) {
    assert.ok(guide.includes(required), `missing guide contract: ${required}`)
  }
})
