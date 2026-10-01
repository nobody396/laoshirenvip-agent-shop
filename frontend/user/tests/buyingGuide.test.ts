import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import { guideStep, guideProducts, guideProductUrl } from '../src/utils/buyingGuide.ts'

test('every reachable answer terminates safely and targets an exact agent SKU', async () => {
  const catalogs = await Promise.all(['zh-CN','zh-TW','en-US'].map(async l => JSON.parse(await readFile(new URL(`../src/i18n/locales/${l}.json`,import.meta.url),'utf8'))))
  const visit = (path: string[]) => {
    assert.ok(path.length <= 7)
    const node = guideStep(path)
    const keys = node.kind === 'question' ? [node.id,...node.options] : node.kind === 'stop' ? [node.reason,`${node.reason}_body`] : ['result',...(node.warning ? [node.warning,`${node.warning}_accept`] : [])]
    for (const c of catalogs) for (const k of keys) assert.ok(c[`guide_${k}`],k)
    if (node.kind === 'question') { assert.ok(node.options.length > 1); for(const option of node.options) visit([...path,option]); return }
    if (node.kind === 'result') {
      assert.ok(guideProducts[node.product]); assert.equal(guideProductUrl(node.product),`/products/${guideProducts[node.product].slug}?sku=${guideProducts[node.product].itemId}`)
      assert.ok(!guideProductUrl(node.product).includes('laoshirenvip.com'))
      assert.ok(!path.includes('after_expiry'))
      if (['plusph','fiveph','twentynew'].includes(node.product)) assert.equal(path[1],'free')
      if (node.product==='twentyrenew') { assert.deepEqual(path.slice(0,3),['gpt','pro200','current_ph']); assert.deepEqual(path.slice(-2),['yes_8919','ph_renew']) }
    }
  }; visit([])
})
test('current iOS Pro200 cannot be misrouted to PH; current PH offers renewal or explicit overwrite',()=>{
  assert.deepEqual(guideStep(['gpt','pro200','current_ios','twenty','recharge_now']),{kind:'result',product:'gpt20ios',warning:'overwrite30'})
  const path=['gpt','pro200','current_ph','twenty','recharge_now','yes_8919']
  assert.equal(guideStep(path).kind,'question')
  assert.equal((guideStep([...path,'ph_renew']) as {product:string}).product,'twentyrenew')
  assert.equal((guideStep([...path,'ios']) as {product:string}).product,'gpt20ios')
  assert.equal(guideProducts.twentyrenew.itemId,29); assert.equal(guideProducts.twentyrenew.productId,35)
  assert.equal(guideProducts.gpt20ios.itemId,4); assert.equal(guideProducts.twentynew.itemId,24)
})
test('both page templates use exact requested SKU with no silent fallback, and the shared notice owns one guide',async()=>{
  const source=await readFile(new URL('../src/composables/useProductDetail.ts',import.meta.url),'utf8')
  assert.match(source,/hasRequestedSku\.value\) \{/); assert.match(source,/requestedSkuId\.value : 0/)
  assert.match(source,/\(hasRequestedSku\.value \|\| activeSkus\.value\.length > 1\) && !selectedSku\.value/)
  assert.match(source,/watch\(\(\) => route\.query\.sku/)
  const notice=await readFile(new URL('../src/components/StorefrontPurchaseNotice.vue',import.meta.url),'utf8')
  assert.equal((notice.match(/<BuyingGuide\s*\/>/g)||[]).length,1)
  for(const f of ['../src/views/ProductDetail.vue','../src/templates/vault/ProductDetail.vue']) assert.match(await readFile(new URL(f,import.meta.url),'utf8'),/id="purchase-options"/)
})
test('result guards retain actual stock, sale flags, consent and stale-response cancellation',async()=>{
  const source=await readFile(new URL('../src/components/BuyingGuide.vue',import.meta.url),'utf8')
  for(const value of ['resolveSkuAvailableStock','sku.value.sale_disabled','accepted.value','seq !== request','s.is_active','data?.id !== reference.productId']) assert.ok(source.includes(value),value)
  assert.match(source,/prefers-reduced-motion:no-preference/)
  assert.match(source,/not\(:focus-visible\)/)
  assert.doesNotMatch(source,/addToCart|buyNow\(/)
})
