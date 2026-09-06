import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const read = (path: string) => readFile(new URL(path, import.meta.url), 'utf8')

test('home purchase notice is white-label on both main and reseller sites', async () => {
  const component = await read('../src/components/StorefrontPurchaseNotice.vue')
  const locale = await read('../src/i18n/locales/zh-CN.json')
  for (const required of [
    '24 小时自动交付',
    '菲区和 iOS 有什么区别',
    '安卓设备、苹果设备还是电脑',
    'CDK 卡密和使用网址',
    '30 天质保',
  ]) {
    assert.ok(locale.includes(required), `missing purchase notice copy: ${required}`)
  }
  assert.match(component, /appStore\.config\?\.contact/)
  assert.match(component, /contact\.telegram/)
  assert.match(component, /contact\.whatsapp/)
  assert.match(component, /contact\.email/)
  assert.match(component, /contact\.support_url/)
  assert.match(component, /v-for="link in supportLinks"/)
  assert.doesNotMatch(component, /v-if="!appStore\.isResellerTenant"/)
})

test('all storefront layouts render every configured support channel', async () => {
  const sources = await Promise.all([
    read('../src/components/Footer.vue'),
    read('../src/views/About.vue'),
    read('../src/templates/vault/About.vue'),
    read('../src/templates/vault/layout/VaultLayout.vue'),
  ])
  for (const source of sources) {
    for (const required of ['telegram', 'whatsapp', 'Email', 'Support']) {
      assert.ok(source.includes(required), `support surface missing ${required}`)
    }
  }
})

test('catalog copy is white-label and Claude products include the complete preflight guide', async () => {
  const products = JSON.parse(await read('../../../catalog/agent-products.json')) as Array<{
    title: Record<string, string>
    description: Record<string, string>
    content: Record<string, string>
  }>
  const serialized = JSON.stringify(products)
  assert.equal(serialized.includes('老实人VIP'), false)
  assert.equal(serialized.includes('高峰期可能延迟'), false)

  const claude = products.filter((product) => product.title['zh-CN']?.includes('Claude'))
  assert.ok(claude.length > 0)
  for (const product of claude) {
    const description = product.description['zh-CN'] || ''
    const content = product.content['zh-CN'] || ''
    assert.ok(description.includes('不懂防封策略的请勿购买'))
    for (const required of [
      '情况一：当前订阅尚未结束',
      '情况二：Billing 存在欠费或退款记录',
      '情况三：组织已被禁用，但仍可登录',
      '正常情况（示例）',
      '异常情况（不可充值）',
      'https://ip-check.leeguoo.com/',
      'https://claude.ai/',
      'https://claude.ai/upgrade?from=menu',
      '/uploads/agent-products/claude-subscription-active.png',
      '/uploads/agent-products/claude-message-normal.png',
      '/uploads/agent-products/claude-org-blocked-chat.png',
      '/uploads/agent-products/claude-upgrade-plans-normal.png',
      '/uploads/agent-products/claude-upgrade-checkout-normal.png',
      '/uploads/agent-products/claude-org-blocked-upgrade.png',
    ]) {
      assert.ok(content.includes(required), `${product.title['zh-CN']} missing: ${required}`)
    }
  }
})
