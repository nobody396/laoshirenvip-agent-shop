<template>
  <div class="space-y-6">
    <ResellerSectionHeader
      :title="t('resellerConsole.customers.title')"
      :description="t('resellerConsole.customers.description')"
    />

    <Card class="border-primary/20 bg-primary/5 p-5">
      <div class="flex gap-3">
        <ShieldCheck class="mt-0.5 h-5 w-5 shrink-0 text-primary" />
        <p class="text-sm text-foreground">{{ t('resellerConsole.customers.scopeHint') }}</p>
      </div>
    </Card>

    <ResellerFilterBar @search="load(1)" @reset="resetFilters">
      <template #fields>
        <Input
          v-model="keyword"
          class="lg:col-span-3"
          :placeholder="t('resellerConsole.customers.searchPlaceholder')"
        />
      </template>
    </ResellerFilterBar>

    <ResellerPageState
      v-if="loading"
      loading
      :title="t('resellerConsole.common.loading')"
    />
    <ResellerPageState
      v-else-if="error"
      :title="error"
      :action-label="t('resellerConsole.common.retry')"
      @action="load(pagination.page)"
    />
    <ResellerPageState
      v-else-if="rows.length === 0"
      :title="t('resellerConsole.customers.empty')"
      :icon="UsersRound"
    />

    <template v-else>
      <Card class="overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{{ t('resellerConsole.customers.account') }}</TableHead>
              <TableHead>{{ t('resellerConsole.customers.status') }}</TableHead>
              <TableHead class="text-right">{{ t('resellerConsole.customers.balance') }}</TableHead>
              <TableHead class="text-right">{{ t('resellerConsole.customers.actions') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="row in rows" :key="row.id">
              <TableCell>
                <div class="font-semibold text-foreground">{{ row.display_name || row.email }}</div>
                <div class="text-xs text-muted-foreground">{{ row.email }}</div>
              </TableCell>
              <TableCell>{{ row.status }}</TableCell>
              <TableCell class="text-right font-mono font-bold">¥{{ money(row.wallet_balance) }}</TableCell>
              <TableCell class="text-right">
                <div class="flex justify-end gap-2">
                <Button size="sm" variant="outline" as-child>
                  <RouterLink :to="`/reseller/customers/${row.id}/prices`">
                    <Tags class="h-4 w-4" />
                    {{ t('resellerConsole.customers.specialPrices') }}
                  </RouterLink>
                </Button>
                <Button size="sm" @click="selectCustomer(row)">
                  <WalletCards class="h-4 w-4" />
                  {{ t('resellerConsole.customers.topUp') }}
                </Button>
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </Card>

      <div class="flex items-center justify-end gap-2">
        <Button variant="outline" size="sm" :disabled="pagination.page <= 1" @click="load(pagination.page - 1)">
          {{ t('resellerConsole.customers.previous') }}
        </Button>
        <span class="text-sm text-muted-foreground">{{ pagination.page }} / {{ pagination.total_page }}</span>
        <Button variant="outline" size="sm" :disabled="pagination.page >= pagination.total_page" @click="load(pagination.page + 1)">
          {{ t('resellerConsole.customers.next') }}
        </Button>
      </div>
    </template>

    <Card v-if="selected" class="p-5">
      <div class="flex items-start justify-between gap-4">
        <div>
          <h2 class="font-bold text-foreground">{{ t('resellerConsole.customers.topUpTitle') }}</h2>
          <p class="mt-1 text-sm text-muted-foreground">{{ selected.email }}</p>
        </div>
        <Button variant="outline" size="sm" @click="selected = null">{{ t('common.cancel') }}</Button>
      </div>
      <div class="mt-5 grid gap-4 sm:grid-cols-2">
        <div>
          <Label for="customer-wallet-amount">{{ t('resellerConsole.customers.amount') }}</Label>
          <Input id="customer-wallet-amount" v-model="form.amount" class="mt-2" inputmode="decimal" placeholder="100.00" />
        </div>
        <div>
          <Label for="customer-wallet-remark">{{ t('resellerConsole.customers.remark') }}</Label>
          <Input id="customer-wallet-remark" v-model="form.remark" class="mt-2" :placeholder="t('resellerConsole.customers.remarkPlaceholder')" />
        </div>
      </div>
      <p class="mt-3 text-xs text-muted-foreground">{{ t('resellerConsole.customers.debitHint') }}</p>
      <div class="mt-4 flex justify-end">
        <Button :disabled="submitting" @click="submitTopUp">
          {{ submitting ? t('resellerConsole.customers.submitting') : t('resellerConsole.customers.confirmTopUp') }}
        </Button>
      </div>
      <p v-if="success" class="mt-3 text-sm font-semibold text-success">{{ success }}</p>
      <p v-if="submitError" class="mt-3 text-sm text-destructive">{{ submitError }}</p>
      <div class="mt-6 border-t pt-5">
        <h3 class="text-sm font-bold text-foreground">{{ t('resellerConsole.customers.transactions') }}</h3>
        <p v-if="transactions.length === 0" class="mt-3 text-sm text-muted-foreground">{{ t('resellerConsole.customers.noTransactions') }}</p>
        <div v-else class="mt-3 space-y-2">
          <div v-for="transaction in transactions" :key="transaction.id" class="flex items-center justify-between gap-4 rounded-lg border p-3 text-sm">
            <div>
              <div class="font-semibold text-foreground">{{ transaction.remark || transaction.type }}</div>
              <div class="text-xs text-muted-foreground">{{ new Date(transaction.created_at).toLocaleString() }}</div>
            </div>
            <div class="text-right font-mono">
              <div :class="transaction.direction === 'in' ? 'text-success' : 'text-destructive'">
                {{ transaction.direction === 'in' ? '+' : '-' }}¥{{ money(transaction.amount) }}
              </div>
              <div class="text-xs text-muted-foreground">¥{{ money(transaction.balance_after) }}</div>
            </div>
          </div>
        </div>
      </div>
    </Card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ShieldCheck, Tags, UsersRound, WalletCards } from 'lucide-vue-next'
import { resellerAPI } from '../../api/reseller'
import type { ResellerCustomerWalletRow, ResellerCustomerWalletTransaction } from '../../api/types'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import ResellerFilterBar from '../../components/reseller-console/ResellerFilterBar.vue'
import ResellerPageState from '../../components/reseller-console/ResellerPageState.vue'
import ResellerSectionHeader from '../../components/reseller-console/ResellerSectionHeader.vue'

const { t } = useI18n()
const loading = ref(true)
const error = ref('')
const keyword = ref('')
const rows = ref<ResellerCustomerWalletRow[]>([])
const selected = ref<ResellerCustomerWalletRow | null>(null)
const transactions = ref<ResellerCustomerWalletTransaction[]>([])
const submitting = ref(false)
const success = ref('')
const submitError = ref('')
const pagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })
const form = reactive({ amount: '', remark: '', request_id: '' })

const money = (value: string) => Number(value || 0).toFixed(2)

const load = async (page = 1) => {
  loading.value = true
  error.value = ''
  try {
    const response = await resellerAPI.customers({ page, page_size: pagination.value.page_size, keyword: keyword.value || undefined })
    rows.value = response.data.data || []
    pagination.value = response.data.pagination || pagination.value
  } catch (err: any) {
    rows.value = []
    error.value = err?.message || t('resellerConsole.common.loadFailed')
  } finally {
    loading.value = false
  }
}

const resetFilters = () => {
  keyword.value = ''
  void load(1)
}

const selectCustomer = (row: ResellerCustomerWalletRow) => {
  selected.value = row
  form.amount = ''
  form.remark = ''
  form.request_id = requestId()
  success.value = ''
  submitError.value = ''
  void loadTransactions(row.id)
}

const loadTransactions = async (customerId: number) => {
  try {
    const response = await resellerAPI.customerWalletTransactions(customerId, { page: 1, page_size: 20 })
    transactions.value = response.data.data || []
  } catch {
    transactions.value = []
  }
}

const requestId = () => {
  const uuid = globalThis.crypto?.randomUUID?.()
  return uuid ? `reseller-topup-${uuid}` : `reseller-topup-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

const submitTopUp = async () => {
  const customer = selected.value
  const amountText = form.amount.trim()
  const amount = Number(amountText)
  if (!customer || !/^(?:0|[1-9]\d*)(?:\.\d{1,2})?$/.test(amountText) || !Number.isFinite(amount) || amount <= 0) {
    submitError.value = t('resellerConsole.customers.invalidAmount')
    return
  }
  if (!window.confirm(t('resellerConsole.customers.confirmPrompt', { email: customer.email, amount: amount.toFixed(2) }))) return
  submitting.value = true
  submitError.value = ''
  success.value = ''
  try {
    const response = await resellerAPI.topUpCustomerWallet(customer.id, {
      amount: amount.toFixed(2),
      request_id: form.request_id,
      remark: form.remark.trim() || undefined,
    })
    const result = response.data.data
    customer.wallet_balance = String(result.balance_after)
    success.value = t('resellerConsole.customers.success', {
      amount: amount.toFixed(2),
      balance: money(customer.wallet_balance),
      ownerBalance: money(String(result.owner_wallet_balance)),
    })
    form.amount = ''
    form.request_id = requestId()
    await loadTransactions(customer.id)
  } catch (err: any) {
    submitError.value = err?.message || t('resellerConsole.customers.failed')
  } finally {
    submitting.value = false
  }
}

onMounted(() => load())
</script>
