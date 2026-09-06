<template>
  <section class="mx-auto w-full max-w-[1180px] px-4 sm:px-6" :class="headerOffset ? 'pt-24' : 'pt-5'">
    <div class="flex flex-col gap-3 rounded-2xl border border-primary/25 bg-primary/5 px-4 py-3 text-sm sm:flex-row sm:items-center sm:justify-between">
      <div class="flex items-start gap-2.5">
        <Clock3 class="mt-0.5 h-4 w-4 shrink-0 text-primary" />
        <p class="leading-6 text-foreground">
          <strong>{{ t('home.purchaseNotice.deliveryTitle') }}</strong>
          {{ t('home.purchaseNotice.deliveryText') }}
        </p>
      </div>
      <div v-if="supportLinks.length" class="flex shrink-0 flex-wrap items-center gap-3 self-start sm:self-auto">
        <a
          v-for="link in supportLinks"
          :key="link.href"
          :href="link.href"
          :target="link.external ? '_blank' : undefined"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1.5 font-semibold text-primary hover:underline"
        >
          {{ link.label }}
          <ExternalLink class="h-3.5 w-3.5" />
        </a>
      </div>
    </div>

    <div class="mt-4 overflow-hidden rounded-2xl border bg-card shadow-sm">
      <div class="border-b bg-muted/35 px-5 py-4 sm:px-6">
        <div class="flex items-center gap-2.5">
          <CircleAlert class="h-5 w-5 text-warning" />
          <h2 class="text-lg font-bold text-foreground">{{ t('home.purchaseNotice.guideTitle') }}</h2>
        </div>
        <p class="mt-1.5 text-sm text-muted-foreground">{{ t('home.purchaseNotice.guideSubtitle') }}</p>
      </div>
      <div class="grid gap-px bg-border sm:grid-cols-2 lg:grid-cols-4">
        <article v-for="item in guideItems" :key="item.title" class="bg-card p-5">
          <component :is="item.icon" class="h-5 w-5 text-primary" />
          <h3 class="mt-3 font-bold text-foreground">{{ item.title }}</h3>
          <p class="mt-1.5 text-sm leading-6 text-muted-foreground">{{ item.text }}</p>
        </article>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, Clock3, ExternalLink, KeyRound, ShieldCheck, Smartphone, WalletCards } from 'lucide-vue-next'
import { useAppStore } from '../stores/app'

withDefaults(defineProps<{ headerOffset?: boolean }>(), { headerOffset: false })

const { t } = useI18n()
const appStore = useAppStore()

const guideItems = computed(() => [
  { icon: WalletCards, title: t('home.purchaseNotice.channelTitle'), text: t('home.purchaseNotice.channelText') },
  { icon: Smartphone, title: t('home.purchaseNotice.deviceTitle'), text: t('home.purchaseNotice.deviceText') },
  { icon: KeyRound, title: t('home.purchaseNotice.usageTitle'), text: t('home.purchaseNotice.usageText') },
  { icon: ShieldCheck, title: t('home.purchaseNotice.warrantyTitle'), text: t('home.purchaseNotice.warrantyText') },
])

const supportLinks = computed(() => {
  const contact = appStore.config?.contact || {}
  const email = String(contact.email || '').trim().replace(/^mailto:/i, '')
  return [
    { label: 'Telegram', href: String(contact.telegram || '').trim() },
    { label: 'WhatsApp', href: String(contact.whatsapp || '').trim() },
    { label: 'Email', href: email ? `mailto:${email}` : '' },
    { label: t('home.purchaseNotice.contactSupport'), href: String(contact.support_url || '').trim() },
  ].filter((item) => item.href).map((item) => ({ ...item, external: /^https?:\/\//i.test(item.href) }))
})
</script>
