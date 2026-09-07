<template>
  <div class="min-h-screen bg-background pb-20 pt-24 text-foreground">
    <div class="container mx-auto max-w-6xl px-4">
      <section class="relative overflow-hidden rounded-[2rem] border bg-card px-6 py-10 shadow-xl md:px-12 md:py-14">
        <div class="pointer-events-none absolute -right-24 -top-24 h-72 w-72 rounded-full bg-primary/10 blur-3xl" />
        <div class="relative max-w-4xl">
          <div class="mb-5 flex flex-wrap gap-2">
            <span class="rounded-full border bg-secondary px-3 py-1 text-xs font-bold text-muted-foreground">{{ t('integrationGuide.badges.dujiao') }}</span>
            <span class="rounded-full border bg-secondary px-3 py-1 text-xs font-bold text-muted-foreground">{{ t('integrationGuide.badges.acg') }}</span>
            <span class="rounded-full border bg-secondary px-3 py-1 text-xs font-bold text-muted-foreground">{{ t('integrationGuide.badges.prepaid') }}</span>
          </div>
          <p class="mb-3 text-sm font-bold uppercase tracking-[0.18em] text-primary">{{ t('integrationGuide.eyebrow') }}</p>
          <h1 class="max-w-3xl text-4xl font-black tracking-tight md:text-6xl">{{ t('integrationGuide.title') }}</h1>
          <p class="mt-5 max-w-3xl text-base leading-8 text-muted-foreground md:text-lg">
            {{ t('integrationGuide.intro') }}
          </p>
          <div class="mt-7 flex flex-wrap gap-3">
            <button class="rounded-xl bg-primary px-5 py-3 text-sm font-bold text-primary-foreground transition hover:bg-primary/90" @click="copyText(guideText, 'guide')">
              <ClipboardCheck v-if="copied === 'guide'" class="mr-2 inline h-4 w-4" />
              <Copy v-else class="mr-2 inline h-4 w-4" />
              {{ copied === 'guide' ? t('integrationGuide.copy.guideCopied') : t('integrationGuide.copy.guide') }}
            </button>
            <button class="rounded-xl border bg-background px-5 py-3 text-sm font-bold transition hover:bg-secondary" @click="copyText(aiPrompt, 'ai')">
              <ClipboardCheck v-if="copied === 'ai'" class="mr-2 inline h-4 w-4" />
              <Bot v-else class="mr-2 inline h-4 w-4" />
              {{ copied === 'ai' ? t('integrationGuide.copy.aiCopied') : t('integrationGuide.copy.ai') }}
            </button>
          </div>
          <div class="mt-7 rounded-xl border border-dashed bg-secondary/50 px-4 py-3 font-mono text-sm text-muted-foreground">
            {{ t('integrationGuide.siteAddress') }}<span class="break-all text-foreground">{{ siteBase }}</span>
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
            <h2 class="text-2xl font-black">{{ t('integrationGuide.preparation.title') }}</h2>
            <p class="mt-2 text-muted-foreground">{{ t('integrationGuide.preparation.description') }}</p>
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
          <p><strong>{{ t('integrationGuide.preparation.secretWarningStrong') }}</strong>{{ t('integrationGuide.preparation.secretWarningBody') }}</p>
        </div>
      </section>

      <section class="mt-10 grid gap-6 lg:grid-cols-2">
        <article class="rounded-2xl border bg-card p-6 md:p-9">
          <div class="flex items-center gap-3">
            <Server class="h-7 w-7 text-primary" />
            <div>
              <p class="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">Dujiao OpenAPI</p>
              <h2 class="text-2xl font-black">{{ t('integrationGuide.dujiao.title') }}</h2>
            </div>
          </div>
          <p class="mt-5 text-sm leading-7 text-muted-foreground">{{ t('integrationGuide.dujiao.intro') }}</p>
          <dl class="mt-5 overflow-hidden rounded-xl border text-sm">
            <div v-for="row in dujiaoFields" :key="row[0]" class="grid grid-cols-[8rem_1fr] border-b last:border-0">
              <dt class="bg-secondary/70 px-4 py-3 font-bold">{{ row[0] }}</dt>
              <dd class="break-all px-4 py-3 font-mono text-xs leading-6">{{ row[1] }}</dd>
            </div>
          </dl>
          <h3 class="mt-7 font-black">{{ t('integrationGuide.dujiao.endpointsTitle') }}</h3>
          <pre class="code-block">POST /api/v1/upstream/ping
GET  /api/v1/upstream/categories
GET  /api/v1/upstream/products
GET  /api/v1/upstream/products/:id
POST /api/v1/upstream/orders
GET  /api/v1/upstream/orders/:id
POST /api/v1/upstream/orders/:id/cancel</pre>
          <h3 class="mt-7 font-black">{{ t('integrationGuide.dujiao.signatureTitle') }}</h3>
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
              <h2 class="text-2xl font-black">{{ t('integrationGuide.acg.title') }}</h2>
            </div>
          </div>
          <p class="mt-5 text-sm leading-7 text-muted-foreground">{{ t('integrationGuide.acg.intro') }}</p>
          <dl class="mt-5 overflow-hidden rounded-xl border text-sm">
            <div v-for="row in acgFields" :key="row[0]" class="grid grid-cols-[8rem_1fr] border-b last:border-0">
              <dt class="bg-secondary/70 px-4 py-3 font-bold">{{ row[0] }}</dt>
              <dd class="break-all px-4 py-3 font-mono text-xs leading-6">{{ row[1] }}</dd>
            </div>
          </dl>
          <h3 class="mt-7 font-black">{{ t('integrationGuide.acg.pathsTitle') }}</h3>
          <pre class="code-block">/shared/authentication/connect
/shared/commodity/items
/shared/commodity/item
/shared/commodity/inventory
/shared/commodity/inventoryState
/shared/commodity/valuation
/shared/commodity/trade
/shared/commodity/query

/plugin/SharedStock/api/*  {{ t('integrationGuide.acg.legacyCompatibility') }}</pre>
          <h3 class="mt-7 font-black">{{ t('integrationGuide.acg.signingTitle') }}</h3>
          <pre class="code-block">{{ acgSigningRules }}</pre>
        </article>
      </section>

      <section class="mt-10 rounded-2xl border bg-card p-6 md:p-9">
        <div class="flex items-start justify-between gap-4">
          <div>
            <p class="text-xs font-bold uppercase tracking-[0.16em] text-primary">Idempotency first</p>
            <h2 class="mt-2 text-2xl font-black">{{ t('integrationGuide.orders.title') }}</h2>
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
            <h2 class="mt-2 text-2xl font-black">{{ t('integrationGuide.ai.title') }}</h2>
            <p class="mt-2 text-sm text-neutral-400">{{ t('integrationGuide.ai.description') }}</p>
          </div>
          <button class="shrink-0 rounded-xl bg-white px-5 py-3 text-sm font-bold text-neutral-950 hover:bg-neutral-200" @click="copyText(aiPrompt, 'ai-bottom')">
            {{ copied === 'ai-bottom' ? t('integrationGuide.ai.copied') : t('integrationGuide.ai.copy') }}
          </button>
        </div>
        <pre class="mt-6 max-h-[34rem] overflow-auto whitespace-pre-wrap rounded-xl border border-white/10 bg-black/40 p-5 text-xs leading-6 text-neutral-300">{{ aiPrompt }}</pre>
      </section>

      <section class="mt-10 rounded-2xl border bg-card p-6 md:p-9">
        <h2 class="text-2xl font-black">{{ t('integrationGuide.checklist.title') }}</h2>
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
import { useI18n } from 'vue-i18n'
import { Bot, Boxes, CircleCheckBig, ClipboardCheck, Copy, Server, ShoppingBag, Store, TriangleAlert, Workflow } from 'lucide-vue-next'
import { usePageSeo } from '../composables/usePageSeo'

const copied = ref('')
const { t } = useI18n()
const siteBase = computed(() => typeof window === 'undefined' ? '' : window.location.origin)

const paths = computed(() => [
  { title: t('integrationGuide.paths.noStore.title'), text: t('integrationGuide.paths.noStore.text'), icon: Store },
  { title: t('integrationGuide.paths.dujiao.title'), text: t('integrationGuide.paths.dujiao.text'), icon: Server },
  { title: t('integrationGuide.paths.acg.title'), text: t('integrationGuide.paths.acg.text'), icon: ShoppingBag },
])

const commonSteps = computed(() => Array.from({ length: 6 }, (_, index) => t(`integrationGuide.preparation.steps.${index + 1}`)))

const dujiaoFields = computed(() => [
  [t('integrationGuide.dujiao.fields.protocol'), 'Dujiao OpenAPI'],
  [t('integrationGuide.dujiao.fields.siteAddress'), siteBase.value],
  ['API Key', 'YOUR_API_KEY'],
  ['API Secret', 'YOUR_API_SECRET'],
  [t('integrationGuide.dujiao.fields.callbackAddress'), t('integrationGuide.dujiao.fields.callbackValue')],
])

const acgFields = computed(() => [
  [t('integrationGuide.acg.fields.siteAddress'), siteBase.value],
  [t('integrationGuide.acg.fields.apiType'), t('integrationGuide.acg.fields.apiTypeValue')],
  [t('integrationGuide.acg.fields.merchantId'), 'YOUR_API_KEY'],
  ['App Key', 'YOUR_API_SECRET'],
  [t('integrationGuide.acg.fields.purchaseMethod'), t('integrationGuide.acg.fields.balancePayment')],
])

const acgSigningRules = computed(() => t('integrationGuide.acg.signingRules'))

const orderRules = computed(() => Array.from({ length: 4 }, (_, index) => ({
  title: t(`integrationGuide.orders.rules.${index + 1}.title`),
  text: t(`integrationGuide.orders.rules.${index + 1}.text`),
})))

const checklist = computed(() => Array.from({ length: 6 }, (_, index) => t(`integrationGuide.checklist.items.${index + 1}`)))

const guideText = computed(() => t('integrationGuide.guideText', { siteBase: siteBase.value }))
const aiPrompt = computed(() => t('integrationGuide.ai.prompt', { siteBase: siteBase.value }))

async function copyText(value: string | { value: string }, key: string) {
  const text = typeof value === 'string' ? value : value.value
  await navigator.clipboard.writeText(text)
  copied.value = key
  window.setTimeout(() => {
    if (copied.value === key) copied.value = ''
  }, 1800)
}

usePageSeo({
  title: () => t('integrationGuide.seo.title'),
  description: () => t('integrationGuide.seo.description'),
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
