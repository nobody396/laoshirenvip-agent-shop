<template>
  <div class="space-y-6">
    <ResellerSectionHeader
      :title="t('resellerConsole.customerPrices.title')"
      :description="customer ? `${customer.display_name || customer.email} · ${customer.email}` : t('resellerConsole.customerPrices.description')"
    >
      <template #actions>
        <Button variant="outline" as-child>
          <RouterLink to="/reseller/customers">{{ t('resellerConsole.customerPrices.back') }}</RouterLink>
        </Button>
      </template>
    </ResellerSectionHeader>

    <Card class="border-primary/20 bg-primary/5 p-5 text-sm text-foreground">
      {{ t('resellerConsole.customerPrices.hint') }}
    </Card>

    <ResellerPageState v-if="loading" loading :title="t('resellerConsole.common.loading')" />
    <ResellerPageState v-else-if="error" :title="error" :action-label="t('resellerConsole.common.retry')" @action="load" />
    <ResellerPageState v-else-if="rows.length === 0" :title="t('resellerConsole.customerPrices.empty')" :icon="Tags" />

    <div v-else class="space-y-4">
      <Card v-for="row in rows" :key="row.product.id" class="overflow-hidden">
        <div class="border-b bg-muted/20 px-5 py-4">
          <h2 class="font-bold text-foreground">{{ productTitle(row.product.title) }}</h2>
          <p class="mt-1 text-xs text-muted-foreground">{{ row.product.slug }}</p>
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{{ t('resellerConsole.customerPrices.sku') }}</TableHead>
              <TableHead class="text-right">{{ t('resellerConsole.customerPrices.basePrice') }}</TableHead>
              <TableHead class="text-right">{{ t('resellerConsole.customerPrices.retailPrice') }}</TableHead>
              <TableHead>{{ t('resellerConsole.customerPrices.specialPrice') }}</TableHead>
              <TableHead class="text-right">{{ t('resellerConsole.customerPrices.grossProfit') }}</TableHead>
              <TableHead class="text-right">{{ t('resellerConsole.customers.actions') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="sku in listedSkus(row)" :key="sku.id">
              <TableCell>
                <div class="font-medium">{{ skuLabel(sku) }}</div>
                <div class="text-xs text-muted-foreground">{{ sku.sku_code }}</div>
              </TableCell>
              <TableCell class="text-right font-mono">¥{{ money(sku.base_price_amount) }}</TableCell>
              <TableCell class="text-right font-mono">¥{{ money(retailPrice(sku)) }}</TableCell>
              <TableCell class="w-44">
                <Input v-model="drafts[key(row.product.id, sku.id)]" inputmode="decimal" placeholder="122.00" />
                <p v-if="rowErrors[key(row.product.id, sku.id)]" class="mt-1 text-xs text-destructive">{{ rowErrors[key(row.product.id, sku.id)] }}</p>
              </TableCell>
              <TableCell class="text-right font-mono">{{ profitText(row.product.id, sku) }}</TableCell>
              <TableCell class="text-right">
                <div class="flex justify-end gap-2">
                  <Button size="sm" :disabled="savingKey === key(row.product.id, sku.id)" @click="save(row.product.id, sku)">
                    {{ t('resellerConsole.customerPrices.save') }}
                  </Button>
                  <Button v-if="hasSaved(row.product.id, sku.id)" size="sm" variant="outline" :disabled="savingKey === key(row.product.id, sku.id)" @click="reset(row.product.id, sku.id)">
                    {{ t('resellerConsole.customerPrices.reset') }}
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </Card>
    </div>
    <p v-if="notice" class="text-sm font-semibold text-success">{{ notice }}</p>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Tags } from 'lucide-vue-next'
import { resellerAPI } from '../../api/reseller'
import type { ResellerCustomerPriceListData, ResellerProductSettingDetailData, ResellerProductSettingSKUData } from '../../api/types'
import { getLocalizedText } from '../../utils/resellerSiteConfig'
import { formatSkuSpecValues } from '../../utils/sku'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import ResellerPageState from '../../components/reseller-console/ResellerPageState.vue'
import ResellerSectionHeader from '../../components/reseller-console/ResellerSectionHeader.vue'

const route = useRoute()
const { t, locale } = useI18n()
const customerID = Number(route.params.id || 0)
const customer = ref<ResellerCustomerPriceListData['customer'] | null>(null)
const rows = ref<ResellerProductSettingDetailData[]>([])
const saved = reactive<Record<string, string>>({})
const drafts = reactive<Record<string, string>>({})
const rowErrors = reactive<Record<string, string>>({})
const loading = ref(true)
const error = ref('')
const notice = ref('')
const savingKey = ref('')

const key = (productID: number, skuID: number) => `${productID}:${skuID}`
const money = (value: unknown) => Number(value || 0).toFixed(2)
const productTitle = (title: Record<string, string>) => getLocalizedText(title, String(locale.value))
const skuLabel = (sku: ResellerProductSettingSKUData) => formatSkuSpecValues(sku.spec_values, String(locale.value)) || sku.sku_code || `#${sku.id}`
const retailPrice = (sku: ResellerProductSettingSKUData) => sku.effective_price_amount || sku.setting?.effective_price_amount || sku.base_price_amount
const listedSkus = (row: ResellerProductSettingDetailData) => row.product_setting?.is_listed === false ? [] : row.skus.filter((sku) => sku.is_active && sku.setting?.is_listed !== false)
const hasSaved = (productID: number, skuID: number) => Object.prototype.hasOwnProperty.call(saved, key(productID, skuID))

const validate = (productID: number, sku: ResellerProductSettingSKUData) => {
  const k = key(productID, sku.id)
  const raw = String(drafts[k] || '').trim()
  if (!/^(?:0|[1-9]\d*)(?:\.\d{1,2})?$/.test(raw) || Number(raw) <= 0) return t('resellerConsole.customerPrices.invalid')
  if (Number(raw) < Number(sku.base_price_amount)) return t('resellerConsole.customerPrices.belowBase')
  if (Number(raw) > Number(retailPrice(sku))) return t('resellerConsole.customerPrices.aboveRetail')
  return ''
}

const profitText = (productID: number, sku: ResellerProductSettingSKUData) => {
  const value = Number(drafts[key(productID, sku.id)])
  if (!Number.isFinite(value) || value <= 0) return '-'
  return `¥${(value - Number(sku.base_price_amount)).toFixed(2)}`
}

const load = async () => {
  loading.value = true
  error.value = ''
  try {
    const [priceResponse, productResponse] = await Promise.all([
      resellerAPI.customerPrices(customerID),
      resellerAPI.productSettings({ page: 1, page_size: 100, listed: 'listed' }),
    ])
    const priceData = priceResponse.data.data as ResellerCustomerPriceListData
    customer.value = priceData.customer
    rows.value = productResponse.data.data || []
    Object.keys(saved).forEach((item) => delete saved[item])
    Object.keys(drafts).forEach((item) => delete drafts[item])
    for (const setting of priceData.settings || []) {
      const k = key(setting.product_id, setting.sku_id)
      saved[k] = setting.fixed_price_amount
      drafts[k] = setting.fixed_price_amount
    }
  } catch (err: any) {
    error.value = err?.message || t('resellerConsole.common.loadFailed')
  } finally {
    loading.value = false
  }
}

const save = async (productID: number, sku: ResellerProductSettingSKUData) => {
  const k = key(productID, sku.id)
  const validation = validate(productID, sku)
  rowErrors[k] = validation
  if (validation) return
  savingKey.value = k
  notice.value = ''
  try {
    const amount = Number(drafts[k]).toFixed(2)
    await resellerAPI.setCustomerPrice(customerID, productID, sku.id, amount)
    saved[k] = amount
    drafts[k] = amount
    rowErrors[k] = ''
    notice.value = t('resellerConsole.customerPrices.saved')
  } catch (err: any) {
    rowErrors[k] = err?.message || t('resellerConsole.customerPrices.failed')
  } finally {
    savingKey.value = ''
  }
}

const reset = async (productID: number, skuID: number) => {
  const k = key(productID, skuID)
  savingKey.value = k
  notice.value = ''
  try {
    await resellerAPI.resetCustomerPrice(customerID, productID, skuID)
    delete saved[k]
    drafts[k] = ''
    rowErrors[k] = ''
    notice.value = t('resellerConsole.customerPrices.resetDone')
  } catch (err: any) {
    rowErrors[k] = err?.message || t('resellerConsole.customerPrices.failed')
  } finally {
    savingKey.value = ''
  }
}

onMounted(load)
</script>
