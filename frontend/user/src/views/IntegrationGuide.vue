<template>
  <div class="min-h-screen bg-background pb-20 pt-24 text-foreground">
    <div class="container mx-auto max-w-6xl px-4">
      <section class="relative overflow-hidden rounded-[2rem] border bg-card px-6 py-10 shadow-xl md:px-12 md:py-14">
        <div class="pointer-events-none absolute -right-24 -top-24 h-72 w-72 rounded-full bg-primary/10 blur-3xl" />
        <div class="relative max-w-4xl">
          <div class="mb-5 flex flex-wrap gap-2">
            <span class="rounded-full border bg-secondary px-3 py-1 text-xs font-bold text-muted-foreground">独角兽 / Dujiao-Next</span>
            <span class="rounded-full border bg-secondary px-3 py-1 text-xs font-bold text-muted-foreground">异次元 ACG SharedStock</span>
            <span class="rounded-full border bg-secondary px-3 py-1 text-xs font-bold text-muted-foreground">预充值余额采购</span>
          </div>
          <p class="mb-3 text-sm font-bold uppercase tracking-[0.18em] text-primary">Reseller integration</p>
          <h1 class="max-w-3xl text-4xl font-black tracking-tight md:text-6xl">发卡系统对接教程</h1>
          <p class="mt-5 max-w-3xl text-base leading-8 text-muted-foreground md:text-lg">
            已有独角兽或异次元卡网，直接把本站作为上游供货站。下游先充值采购余额，客户订单进来后按代理供货价自动扣款、自动采购并返回交付内容。
          </p>
          <div class="mt-7 flex flex-wrap gap-3">
            <button class="rounded-xl bg-primary px-5 py-3 text-sm font-bold text-primary-foreground transition hover:bg-primary/90" @click="copyText(guideText, 'guide')">
              <ClipboardCheck v-if="copied === 'guide'" class="mr-2 inline h-4 w-4" />
              <Copy v-else class="mr-2 inline h-4 w-4" />
              {{ copied === 'guide' ? '已复制完整教程' : '复制完整教程' }}
            </button>
            <button class="rounded-xl border bg-background px-5 py-3 text-sm font-bold transition hover:bg-secondary" @click="copyText(aiPrompt, 'ai')">
              <ClipboardCheck v-if="copied === 'ai'" class="mr-2 inline h-4 w-4" />
              <Bot v-else class="mr-2 inline h-4 w-4" />
              {{ copied === 'ai' ? '已复制 AI 提示词' : '复制给 AI 的提示词' }}
            </button>
          </div>
          <div class="mt-7 rounded-xl border border-dashed bg-secondary/50 px-4 py-3 font-mono text-sm text-muted-foreground">
            本站地址：<span class="break-all text-foreground">{{ siteBase }}</span>
          </div>
        </div>
      </section>

      <section class="mt-10 grid gap-4 md:grid-cols-3">
        <article v-for="item in paths" :key="item.title" class="rounded-2xl border bg-card p-6 shadow-sm">
          <component :is="item.icon" class="mb-4 h-7 w-7 text-primary" />
          <h2 class="text-xl font-black">{{ item.title }}</h2>
          <p class="mt-3 text-sm leading-7 text-muted-foreground">{{ item.text }}</p>
        </article>
      </section>

      <section class="mt-10 rounded-2xl border bg-card p-6 md:p-9">
        <div class="flex items-start gap-4">
          <div class="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-primary text-sm font-black text-primary-foreground">01</div>
          <div>
            <h2 class="text-2xl font-black">两种系统都要先完成的准备</h2>
            <p class="mt-2 text-muted-foreground">API Key 和 API Secret 对应同一个采购钱包，两套协议不会建立重复余额或重复订单。</p>
          </div>
        </div>
        <ol class="mt-7 grid gap-4 md:grid-cols-2">
          <li v-for="(step, index) in commonSteps" :key="step" class="flex gap-4 rounded-xl bg-secondary/60 p-4">
            <span class="font-mono text-sm font-black text-primary">{{ String(index + 1).padStart(2, '0') }}</span>
            <span class="text-sm leading-7">{{ step }}</span>
          </li>
        </ol>
        <div class="mt-6 flex gap-3 rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm leading-7 text-amber-900 dark:text-amber-200">
          <TriangleAlert class="mt-1 h-5 w-5 shrink-0" />
          <p><strong>不要把真实 API Secret 发给任何 AI 或第三方。</strong>教程可以直接交给 AI，凭证只在你自己的服务器配置中填写。</p>
        </div>
      </section>

      <section class="mt-10 grid gap-6 lg:grid-cols-2">
        <article class="rounded-2xl border bg-card p-6 md:p-9">
          <div class="flex items-center gap-3">
            <Server class="h-7 w-7 text-primary" />
            <div>
              <p class="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">Dujiao OpenAPI</p>
              <h2 class="text-2xl font-black">独角兽 / Dujiao-Next</h2>
            </div>
          </div>
          <p class="mt-5 text-sm leading-7 text-muted-foreground">在下游后台进入“对接管理 → 连接管理 → 新增”，填写下面的字段。</p>
          <dl class="mt-5 overflow-hidden rounded-xl border text-sm">
            <div v-for="row in dujiaoFields" :key="row[0]" class="grid grid-cols-[8rem_1fr] border-b last:border-0">
              <dt class="bg-secondary/70 px-4 py-3 font-bold">{{ row[0] }}</dt>
              <dd class="break-all px-4 py-3 font-mono text-xs leading-6">{{ row[1] }}</dd>
            </div>
          </dl>
          <h3 class="mt-7 font-black">核心接口</h3>
          <pre class="code-block">POST /api/v1/upstream/ping
GET  /api/v1/upstream/categories
GET  /api/v1/upstream/products
GET  /api/v1/upstream/products/:id
POST /api/v1/upstream/orders
GET  /api/v1/upstream/orders/:id
POST /api/v1/upstream/orders/:id/cancel</pre>
          <h3 class="mt-7 font-black">签名规则</h3>
          <pre class="code-block">body_md5 = MD5(raw_request_body)
sign_text = METHOD + "\n" + PATH + "\n" + UNIX_TIMESTAMP + "\n" + body_md5
signature = HMAC_SHA256_HEX(API_SECRET, sign_text)

Dujiao-Next-Api-Key: API_KEY
Dujiao-Next-Timestamp: UNIX_TIMESTAMP
Dujiao-Next-Signature: signature</pre>
        </article>

        <article class="rounded-2xl border bg-card p-6 md:p-9">
          <div class="flex items-center gap-3">
            <Boxes class="h-7 w-7 text-primary" />
            <div>
              <p class="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">ACG SharedStock</p>
              <h2 class="text-2xl font-black">异次元发卡</h2>
            </div>
          </div>
          <p class="mt-5 text-sm leading-7 text-muted-foreground">在异次元后台新增“店铺共享”。优先选择新版 SharedStock；旧版本选择 SharedStock 插件，两套路径本站都支持。</p>
          <dl class="mt-5 overflow-hidden rounded-xl border text-sm">
            <div v-for="row in acgFields" :key="row[0]" class="grid grid-cols-[8rem_1fr] border-b last:border-0">
              <dt class="bg-secondary/70 px-4 py-3 font-bold">{{ row[0] }}</dt>
              <dd class="break-all px-4 py-3 font-mono text-xs leading-6">{{ row[1] }}</dd>
            </div>
          </dl>
          <h3 class="mt-7 font-black">新版 / 旧版路径</h3>
          <pre class="code-block">/shared/authentication/connect
/shared/commodity/items
/shared/commodity/item
/shared/commodity/inventory
/shared/commodity/inventoryState
/shared/commodity/valuation
/shared/commodity/trade
/shared/commodity/query

/plugin/SharedStock/api/*  （旧版兼容）</pre>
          <h3 class="mt-7 font-black">表单签名规则</h3>
          <pre class="code-block">1. 表单携带 app_id、app_key 和业务字段
2. 删除 sign，忽略空值
3. 按字段名升序排列
4. 拼成 key=value&amp;key=value
5. 末尾追加 &amp;key=APP_KEY
6. 对完整字符串计算小写 MD5</pre>
        </article>
      </section>

      <section class="mt-10 rounded-2xl border bg-card p-6 md:p-9">
        <div class="flex items-start justify-between gap-4">
          <div>
            <p class="text-xs font-bold uppercase tracking-[0.16em] text-primary">Idempotency first</p>
            <h2 class="mt-2 text-2xl font-black">订单与重试规则</h2>
          </div>
          <Workflow class="h-8 w-8 text-primary" />
        </div>
        <div class="mt-6 grid gap-4 md:grid-cols-2">
          <div v-for="rule in orderRules" :key="rule.title" class="rounded-xl border bg-secondary/40 p-5">
            <h3 class="font-black">{{ rule.title }}</h3>
            <p class="mt-2 text-sm leading-7 text-muted-foreground">{{ rule.text }}</p>
          </div>
        </div>
      </section>

      <section class="mt-10 rounded-2xl border bg-neutral-950 p-6 text-neutral-100 md:p-9">
        <div class="flex flex-col justify-between gap-5 md:flex-row md:items-center">
          <div>
            <p class="text-xs font-bold uppercase tracking-[0.16em] text-emerald-400">Ready for AI</p>
            <h2 class="mt-2 text-2xl font-black">直接把下面这段丢给你的 AI</h2>
            <p class="mt-2 text-sm text-neutral-400">不包含真实凭证。AI 完成代码后，先在测试环境跑连接、目录和幂等测试。</p>
          </div>
          <button class="shrink-0 rounded-xl bg-white px-5 py-3 text-sm font-bold text-neutral-950 hover:bg-neutral-200" @click="copyText(aiPrompt, 'ai-bottom')">
            {{ copied === 'ai-bottom' ? '已复制' : '复制 AI 提示词' }}
          </button>
        </div>
        <pre class="mt-6 max-h-[34rem] overflow-auto whitespace-pre-wrap rounded-xl border border-white/10 bg-black/40 p-5 text-xs leading-6 text-neutral-300">{{ aiPrompt }}</pre>
      </section>

      <section class="mt-10 rounded-2xl border bg-card p-6 md:p-9">
        <h2 class="text-2xl font-black">上线前检查</h2>
        <div class="mt-6 grid gap-3 md:grid-cols-2">
          <div v-for="item in checklist" :key="item" class="flex gap-3 rounded-xl bg-secondary/50 p-4 text-sm leading-6">
            <CircleCheckBig class="h-5 w-5 shrink-0 text-emerald-500" />
            <span>{{ item }}</span>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { Bot, Boxes, CircleCheckBig, ClipboardCheck, Copy, Server, ShoppingBag, Store, TriangleAlert, Workflow } from 'lucide-vue-next'
import { usePageSeo } from '../composables/usePageSeo'

const copied = ref('')
const siteBase = computed(() => typeof window === 'undefined' ? '' : window.location.origin)

const paths = [
  { title: '没有自己的卡网', text: '申请本站白标子站即可。服务器、上游和支付由总站统一处理，你只负责品牌、售价、推广和售后。', icon: Store },
  { title: '已有独角兽卡网', text: '使用 Dujiao OpenAPI 对接本站，商品同步后由你的站点自主定零售价，订单按采购余额自动扣款。', icon: Server },
  { title: '已有异次元卡网', text: '使用 ACG SharedStock 对接本站。新版与旧版插件路径都兼容，不需要单独开发第二套钱包。', icon: ShoppingBag },
]

const commonSteps = [
  '在本站注册并登录采购账号。',
  '充值采购余额；API 订单只使用余额，不直接调用终端客户的付款渠道。',
  '进入个人中心 → API 对接，提交 API 凭证申请。',
  '管理员审核后保存 API Key；API Secret 只展示一次，丢失后需要重新生成。',
  '下游先测试连接，再同步分类、商品、SKU、价格与库存。',
  '只使用自有测试账号跑一笔最小订单，确认扣款、交付、查询和幂等后再开放销售。',
]

const dujiaoFields = computed(() => [
  ['协议', 'Dujiao OpenAPI'],
  ['站点地址', siteBase.value],
  ['API Key', 'YOUR_API_KEY'],
  ['API Secret', 'YOUR_API_SECRET'],
  ['回调地址', '由你的下游站点生成的 HTTPS 回调地址'],
])

const acgFields = computed(() => [
  ['店铺地址', siteBase.value],
  ['接口类型', '新版 SharedStock；旧站选 SharedStock 插件'],
  ['商户 ID', 'YOUR_API_KEY'],
  ['App Key', 'YOUR_API_SECRET'],
  ['采购方式', '余额支付'],
])

const orderRules = [
  { title: '订单号永久唯一', text: 'Dujiao 使用 downstream_order_no；SharedStock 使用 request_no。同一采购业务永远复用原编号。' },
  { title: '未知结果先查询', text: '超时或“处理中”不代表失败。先查询原订单，禁止换新编号重新下单。' },
  { title: '余额不足不采购', text: '下游余额不足时订单不会进入上游采购。补充余额后应使用原业务编号按协议重试。' },
  { title: '交付以订单状态为准', text: '收到卡密或交付内容后保存到自己的订单。回调失败时通过查询接口轮询补偿。' },
]

const checklist = [
  '正式环境必须使用 HTTPS，服务器时间保持准确。',
  'API Secret 仅保存在服务器密钥管理中，不写进前端或聊天。',
  '完成连接、目录、库存、估价、下单、查询和重复请求测试。',
  '确认相同业务编号只扣一次余额、只采购一次。',
  '设置余额不足、商品下架、库存不足和上游超时告警。',
  '建立订单、钱包、上游采购和交付的每日对账。',
]

const guideText = computed(() => `老实人VIP代理供货站对接教程

本站地址：${siteBase.value}

一、准备
1. 注册采购账号并充值余额。
2. 在个人中心申请 API 凭证，管理员审核后获取 API Key 与 API Secret。
3. API Secret 不得发送给 AI 或第三方，只在自己的服务器配置。

二、独角兽 / Dujiao-Next
协议：Dujiao OpenAPI
站点地址：${siteBase.value}
API Key：YOUR_API_KEY
API Secret：YOUR_API_SECRET
连接后同步分类、商品和 SKU，再用自有测试账号完成最小订单。

三、异次元 ACG
店铺地址：${siteBase.value}
接口类型：优先新版 SharedStock，旧站可选 SharedStock 插件
商户 ID：YOUR_API_KEY
App Key：YOUR_API_SECRET
本站兼容 /shared/* 与 /plugin/SharedStock/api/*。

四、强制规则
- downstream_order_no / request_no 必须永久唯一。
- 超时或未知结果先查原订单，禁止换编号重买。
- 余额不足不采购；所有 API 订单按采购余额结算。
- 正式开放前必须验证扣款、交付、查询和幂等。`)

const aiPrompt = computed(() => `你要把我的发卡系统接入“老实人VIP代理供货站”。请先阅读并遵守以下契约，然后检查现有项目类型，制定最小改动方案并完成测试。不要索要或输出真实 API Secret。

上游根地址：${siteBase.value}
认证占位符：YOUR_API_KEY / YOUR_API_SECRET

如果项目是 Dujiao-Next：
1. 使用 Dujiao OpenAPI。
2. 请求头：Dujiao-Next-Api-Key、Dujiao-Next-Timestamp、Dujiao-Next-Signature。
3. body_md5=MD5(raw body)。签名原文为 METHOD + "\\n" + PATH + "\\n" + timestamp + "\\n" + body_md5，使用 API Secret 做 HMAC-SHA256，输出小写十六进制。
4. 接口：POST /api/v1/upstream/ping；GET /api/v1/upstream/categories；GET /api/v1/upstream/products；GET /api/v1/upstream/products/:id；POST /api/v1/upstream/orders；GET /api/v1/upstream/orders/:id；POST /api/v1/upstream/orders/:id/cancel。
5. 创建订单必须传永久唯一 downstream_order_no。超时先查询原订单，禁止换编号重试。

如果项目是异次元 ACG：
1. 优先使用新版 SharedStock：/shared/authentication/connect 与 /shared/commodity/{items,item,inventory,inventoryState,valuation,trade,query}。
2. 旧版兼容前缀：/plugin/SharedStock/api/*。
3. 使用 application/x-www-form-urlencoded，表单携带 app_id=YOUR_API_KEY、app_key=YOUR_API_SECRET 与业务字段。
4. 签名：删除 sign，忽略空值，按字段名升序，拼接 key=value&key=value，末尾追加 &key=YOUR_API_SECRET，计算小写 MD5。
5. trade 必须传永久唯一 request_no。若返回处理中或请求超时，保持原 request_no 查询或重试，禁止生成新编号。

共同要求：
- 下游使用预充值采购余额；余额不足不得创建上游采购。
- 保存上游订单号、状态和交付内容；回调失败使用查询轮询补偿。
- 商品下架、库存不足、余额不足和未知结果必须分开处理。
- 测试连接、目录、SKU 映射、库存、估价、余额扣款、交付、查询和幂等。
- 给我输出：修改文件列表、配置项、测试结果、尚未验证的真实资金路径。`)

async function copyText(value: string | { value: string }, key: string) {
  const text = typeof value === 'string' ? value : value.value
  await navigator.clipboard.writeText(text)
  copied.value = key
  window.setTimeout(() => {
    if (copied.value === key) copied.value = ''
  }, 1800)
}

usePageSeo({
  title: () => '发卡系统对接教程',
  description: () => '独角兽 Dujiao-Next 与异次元 ACG SharedStock 对接老实人VIP代理供货站的完整教程。',
  canonicalPath: () => '/integration-guide',
})
</script>

<style scoped>
.code-block {
  margin-top: 0.75rem;
  max-width: 100%;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-word;
  border: 1px solid hsl(var(--border));
  border-radius: 0.75rem;
  background: hsl(var(--secondary) / 0.55);
  padding: 1rem;
  font-size: 0.75rem;
  line-height: 1.65;
  color: hsl(var(--muted-foreground));
}
</style>
