<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowLeft, ArrowRight, ExternalLink } from 'lucide-vue-next'
import { DialogRoot, DialogTrigger, DialogPortal, DialogOverlay, DialogContent, DialogTitle, DialogDescription, DialogClose } from 'reka-ui'
import { Button } from './ui/button'
import { productAPI } from '../api/product'
import { useLocalized } from '../composables/useProduct'
import { guideStep, guideProducts, guideProductUrl } from '../utils/buyingGuide'
import { buildSkuDisplayText } from '../utils/sku'
import { resolveSkuAvailableStock } from '../utils/publicStock'
interface GuideSku { id: number; is_active: boolean; sale_disabled?: boolean; price_amount: string; sku_code: string; spec_values: Record<string, unknown>; [key: string]: unknown }
interface GuideCatalog { id: number; is_active?: boolean; sale_disabled?: boolean; skus: GuideSku[]; title: Record<string, string>; min_purchase_quantity?: number; [key: string]: unknown }
const { t, te, locale } = useI18n()
const text = (key: string) => t(`guide_${key}`)
const { getLocalizedText, formatPrice, siteCurrency } = useLocalized()
const open = ref(false), answers = ref<string[]>([]), accepted = ref(false), loading = ref(false), failed = ref(false)
const product = ref<GuideCatalog | null>(null), heading = ref<HTMLElement | null>(null)
const step = computed(() => guideStep(answers.value))
const sku = computed(() => { const node = step.value; return node.kind === 'result' ? product.value?.skus.find(s => s.id === guideProducts[node.product].itemId && s.is_active) : undefined })
const unavailable = computed(() => !sku.value || product.value?.is_active === false || product.value?.sale_disabled || sku.value.sale_disabled)
const available = computed(() => { if (unavailable.value) return false; const stock = resolveSkuAvailableStock(product.value, sku.value); return stock === null || stock >= Number(product.value?.min_purchase_quantity || 1) })
const allowed = computed(() => available.value && (step.value.kind !== 'result' || !step.value.warning || accepted.value))
const title = computed(() => text(step.value.kind === 'question' ? step.value.id : step.value.kind === 'stop' ? step.value.reason : 'result'))
const description = computed(() => { const node = step.value; if (node.kind === 'stop') return text(`${node.reason}_body`); if (node.kind === 'result') return text(node.warning === 'overwrite30' ? 'overwrite_result_help' : 'result_help'); return text(te(`guide_${node.id}_help`) ? `${node.id}_help` : 'flow_help') })
const help = computed(() => step.value.kind === 'question' ? step.value.help : undefined)
const helpUrl = computed(() => help.value === 'billing' ? 'https://chatgpt.com/#settings/Billing' : help.value === 'claude' ? 'https://claude.ai/' : 'https://chatgpt.com/')
const helpImage = computed(() => help.value === 'plan' ? '/guides/buying/current-plan.png' : help.value === 'billing' ? '/guides/buying/billing.png' : '')
const imageOpen = ref(false), zoomed = ref(false)
watch(imageOpen, () => { zoomed.value = false })
watch(open, value => { if (!value) { answers.value = []; imageOpen.value = false } })
watch(() => answers.value.join('/'), async () => { await nextTick(); heading.value?.focus() })
let request = 0
async function loadResult() {
  const seq = ++request; accepted.value = false; product.value = null; failed.value = false; loading.value = false
  const node = step.value
  if (!open.value || node.kind !== 'result') return
  const reference = guideProducts[node.product]; loading.value = true
  try { const response = await productAPI.detail(reference.slug); if (seq !== request) return; const data = response.data.data as GuideCatalog; if (data?.id !== reference.productId || !Array.isArray(data.skus)) throw new Error('invalid catalog'); product.value = data }
  catch { if (seq === request) failed.value = true }
  finally { if (seq === request) loading.value = false }
}
watch(() => [open.value, answers.value.join('/'), locale.value], loadResult)
</script>
<template>
  <DialogRoot v-model:open="open">
    <DialogTrigger as-child><Button class="guide-breathe rounded-full">{{ text('start') }}<ArrowRight class="size-4" /></Button></DialogTrigger>
    <DialogPortal><DialogOverlay class="fixed inset-0 z-[110] bg-black/60" />
      <DialogContent class="fixed left-1/2 top-1/2 z-[111] max-h-[90dvh] w-[calc(100%-2rem)] max-w-xl -translate-x-1/2 -translate-y-1/2 space-y-4 overflow-y-auto rounded-2xl border bg-card p-5 text-foreground shadow-xl sm:p-7">
        <DialogTitle as-child><h2 ref="heading" tabindex="-1" class="pr-8 text-xl font-semibold outline-none">{{ title }}</h2></DialogTitle>
        <DialogDescription class="text-sm leading-6 text-muted-foreground">{{ description }}</DialogDescription><DialogClose class="absolute right-4 top-3 rounded p-1" :aria-label="text('close')">✕</DialogClose>
        <template v-if="step.kind === 'question'">
          <div v-if="help" class="space-y-3 rounded-xl bg-muted/50 p-4 text-sm leading-6">
            <p>{{ text(`${help}_help`) }}</p><Button variant="outline" as-child><a :href="helpUrl" target="_blank" rel="noopener noreferrer">{{ text(help === 'billing' ? 'open_billing' : help === 'claude' ? 'open_claude' : 'open_plan') }}<ExternalLink class="size-4" /></a></Button>
            <p v-if="help === 'plan'" class="font-medium text-amber-700 dark:text-amber-300">{{ text('grace') }}</p>
            <details v-if="helpImage"><summary class="cursor-pointer underline">{{ text('example') }}</summary>
              <DialogRoot v-model:open="imageOpen"><DialogTrigger as-child><button class="mt-3 w-full cursor-zoom-in rounded"><img :src="helpImage" :alt="text(help === 'billing' ? 'billing_alt' : 'plan_alt')" class="w-full rounded" /><span class="block underline">{{ text('enlarge') }}</span></button></DialogTrigger>
                <DialogPortal><DialogOverlay class="fixed inset-0 z-[120] bg-black/70" /><DialogContent class="fixed left-1/2 top-1/2 z-[121] max-h-[95dvh] w-[calc(100%-1rem)] max-w-6xl -translate-x-1/2 -translate-y-1/2 space-y-3 rounded-xl border bg-card p-4 text-foreground">
                  <DialogTitle>{{ text('preview_image') }}</DialogTitle><DialogDescription class="sr-only">{{ text(help === 'billing' ? 'billing_alt' : 'plan_alt') }}</DialogDescription><DialogClose class="absolute right-4 top-3" :aria-label="text('close')">✕</DialogClose>
                  <Button variant="outline" @click="zoomed = !zoomed">{{ text(zoomed ? 'image_fit' : 'image_original') }}</Button><div tabindex="0" class="max-h-[75dvh] overflow-auto"><img :src="helpImage" :alt="text(help === 'billing' ? 'billing_alt' : 'plan_alt')" :class="zoomed ? 'max-w-none' : 'h-auto w-full'" /></div>
                </DialogContent></DialogPortal>
              </DialogRoot>
            </details>
          </div>
          <div class="grid gap-2 sm:grid-cols-2" :class="step.id === 'family' ? 'grid-cols-2' : ''"><button v-for="option in step.options" :key="option" class="min-h-16 rounded-xl border bg-background p-4 text-left transition-colors hover:border-primary hover:bg-primary/5 focus-visible:outline-2 focus-visible:outline-primary" @click="answers = [...answers, option]"><span class="block font-semibold">{{ text(option) }}</span><span v-if="te(`guide_${option}_hint`)" class="mt-1 block text-xs leading-5 text-muted-foreground">{{ text(`${option}_hint`) }}</span></button></div>
        </template>
        <template v-else-if="step.kind === 'result'">
          <p v-if="loading" role="status">{{ text('loading') }}</p><div v-else-if="failed" role="alert"><p>{{ text('load_error') }}</p><Button variant="outline" @click="loadResult">{{ text('retry') }}</Button></div><p v-else-if="unavailable" role="status" class="rounded-xl bg-muted p-4">{{ text('unavailable') }}</p>
          <template v-else-if="sku && product"><div class="rounded-xl border border-primary/30 bg-primary/5 p-5"><h3 class="font-semibold">{{ buildSkuDisplayText({ skuCode: sku.sku_code, specValues: sku.spec_values, locale, fallback: getLocalizedText(product.title) }) }}</h3><p class="mt-3 text-2xl font-bold text-primary">{{ formatPrice(sku.price_amount, siteCurrency) }}</p><p class="mt-2 text-sm" role="status">{{ text(available ? 'available' : 'sold_out') }}</p></div>
            <div v-if="step.warning" class="space-y-3 rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm leading-6"><p>{{ text(step.warning) }}</p><label class="flex items-start gap-2"><input v-model="accepted" type="checkbox" class="mt-1 size-4" />{{ text(`${step.warning}_accept`) }}</label></div>
            <Button v-if="allowed" as-child class="w-full"><a :href="guideProductUrl(step.product)">{{ text('view') }}<ArrowRight class="size-4" /></a></Button><Button v-else disabled class="w-full">{{ text(available ? 'view' : 'sold_out') }}</Button><p class="text-xs text-muted-foreground">{{ text('live_note') }}</p>
          </template>
        </template>
        <div v-if="answers.length" class="flex justify-between border-t pt-3"><Button variant="ghost" @click="answers = answers.slice(0, -1)"><ArrowLeft class="size-4" />{{ text('back') }}</Button><Button variant="ghost" @click="answers = []">{{ text('restart') }}</Button></div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
<style scoped>
@keyframes guide-breathe { 0%,100% { transform: scale(1); box-shadow: 0 0 0 0 color-mix(in oklab,var(--primary) 32%,transparent); } 50% { transform: scale(1.04); box-shadow: 0 0 0 7px color-mix(in oklab,var(--primary) 18%,transparent); } }
@media(prefers-reduced-motion:no-preference) { .guide-breathe:not(:hover):not(:focus-visible):not([data-state="open"]) { animation:guide-breathe 2.8s ease-in-out infinite; } }
</style>
