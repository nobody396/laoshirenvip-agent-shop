<template>
  <div class="min-h-screen bg-background px-4 pb-16 pt-24 text-foreground">
    <div class="mx-auto max-w-3xl">
      <h1 class="text-3xl font-black">申请电子发票</h1>
      <p class="mt-2 text-sm text-muted-foreground">填写资料并支付开票补款，支付成功后自动进入财务待办</p>
	  <p class="mt-1 text-sm text-muted-foreground">开票项目：生产生活服务信息系统服务</p>

      <div v-if="result" class="mt-8 rounded-2xl border bg-card p-6 shadow-sm">
		<h2 class="text-xl font-bold">{{ result.status === 'pending_payment' ? '请扫码支付' : '开票申请已付款' }}</h2>
		<p v-if="result.status !== 'pending_payment'" class="mt-2 text-sm text-emerald-600">财务将在飞书待办中处理，发票开好后会自动发送到你填写的邮箱</p>
        <div class="mt-5 grid gap-6 md:grid-cols-2">
		  <img v-if="result.status === 'pending_payment' && qrImage" :src="qrImage" alt="支付宝付款二维码" class="mx-auto size-56 rounded-xl bg-white p-3" />
          <div class="space-y-3 text-sm">
            <p>申请编号：<strong>{{ result.request_no }}</strong></p>
            <p>订单金额：¥{{ result.original_amount }}</p>
            <p>开票补款：¥{{ result.invoice_fee_amount }}</p>
            <p>通道手续费（{{ result.payment_fee_rate }}%）：¥{{ result.payment_fee_amount }}</p>
            <p class="text-lg">本次支付：<strong>¥{{ result.payment_amount }}</strong></p>
            <p>发票价税合计：¥{{ result.invoice_total_amount }}</p>
			<a v-if="result.status === 'pending_payment' && result.pay_url" :href="result.pay_url" target="_blank" rel="noopener noreferrer" class="inline-flex rounded-lg bg-primary px-4 py-2 font-semibold text-primary-foreground">打开支付宝付款</a>
          </div>
        </div>
      </div>

      <form v-else class="mt-8 space-y-6 rounded-2xl border bg-card p-6 shadow-sm" @submit.prevent="submit">
        <label class="grid gap-2 text-sm">
		  <span>{{ isRecharge ? '充值单号' : '订单号' }}</span>
          <Input v-model="form.order_no" required />
        </label>
		<div v-if="isGMShop" class="grid gap-2 text-sm"><span>下单邮箱</span><Input v-model="guest.email" type="email" required /></div>
        <div v-else-if="isGuest" class="grid gap-4 md:grid-cols-2">
          <label class="grid gap-2 text-sm"><span>下单邮箱</span><Input v-model="guest.email" type="email" required /></label>
          <label class="grid gap-2 text-sm"><span>订单查询密码</span><Input v-model="guest.order_password" type="password" required /></label>
        </div>
        <div class="grid gap-4 md:grid-cols-2">
          <label class="grid gap-2 text-sm"><span>发票类型</span><select v-model="form.invoice_type" class="h-11 rounded-md border bg-background px-3"><option value="ordinary">普通发票（3%）</option><option value="special">专用发票（6%）</option></select></label>
          <label class="grid gap-2 text-sm"><span>发票抬头</span><Input v-model="form.buyer_title" required /></label>
        </div>
        <label class="grid gap-2 text-sm"><span>统一社会信用代码</span><Input v-model="form.tax_number" required /></label>
        <div v-if="form.invoice_type === 'special'" class="grid gap-4 md:grid-cols-2">
          <label class="grid gap-2 text-sm"><span>公司地址</span><Input v-model="form.company_address" required /></label>
          <label class="grid gap-2 text-sm"><span>公司电话</span><Input v-model="form.company_phone" required /></label>
          <label class="grid gap-2 text-sm"><span>开户银行</span><Input v-model="form.bank_name" required /></label>
          <label class="grid gap-2 text-sm"><span>银行账号</span><Input v-model="form.bank_account" required /></label>
        </div>
        <div class="grid gap-4 md:grid-cols-2">
          <label class="grid gap-2 text-sm"><span>发票接收邮箱</span><Input v-model="form.recipient_email" type="email" required /></label>
          <label class="grid gap-2 text-sm"><span>再次确认邮箱</span><Input v-model="form.confirm_email" type="email" required /></label>
        </div>
        <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
        <Button type="submit" class="w-full" :disabled="loading">{{ loading ? '正在创建支付…' : '确认资料并生成付款码' }}</Button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import QRCode from 'qrcode'
import { invoiceAPI } from '../api/order'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'

const route = useRoute()
const isGuest = computed(() => route.query.guest === '1')
const isGMShop = computed(() => route.query.source === 'gmshop')
const isRecharge = computed(() => route.query.source === 'recharge')
const loading = ref(false)
const error = ref('')
const result = ref<any>(null)
const qrImage = ref('')
const guest = reactive({ email: '', order_password: '' })
const form = reactive({
	order_no: String(route.query.order_no || route.query.recharge_no || ''), invoice_type: 'ordinary', buyer_title: '', tax_number: '',
  company_address: '', company_phone: '', bank_name: '', bank_account: '', recipient_email: '', confirm_email: '',
})
let statusTimer: number | undefined

async function loadRequest(requestNo: string) {
  try {
    result.value = (await invoiceAPI.get(requestNo)).data
    if (result.value.status === 'pending_payment') {
      const qr = result.value.qr_code || result.value.pay_url
      if (qr) qrImage.value = await QRCode.toDataURL(qr, { width: 360, margin: 1 })
    } else if (statusTimer) {
      window.clearInterval(statusTimer)
      statusTimer = undefined
    }
  } catch (cause: any) {
    error.value = cause?.message || '开票申请查询失败'
  }
}

onMounted(() => {
  const requestNo = String(route.query.request_no || '')
  if (!requestNo) return
  void loadRequest(requestNo)
  statusTimer = window.setInterval(() => void loadRequest(requestNo), 3000)
})
onBeforeUnmount(() => { if (statusTimer) window.clearInterval(statusTimer) })

async function submit() {
  error.value = ''
  if (form.recipient_email.trim().toLowerCase() !== form.confirm_email.trim().toLowerCase()) {
    error.value = '两次输入的发票接收邮箱不一致'
    return
  }
  loading.value = true
  try {
	let response
	if (isGMShop.value) response = await invoiceAPI.createGMShop({ ...form, order_email: guest.email })
	else if (isRecharge.value) response = await invoiceAPI.createRecharge(form)
	else if (isGuest.value) response = await invoiceAPI.createGuest({ ...form, ...guest })
	else response = await invoiceAPI.create(form)
    result.value = response.data
    const qr = result.value.qr_code || result.value.pay_url
    if (qr) qrImage.value = await QRCode.toDataURL(qr, { width: 360, margin: 1 })
  } catch (cause: any) {
    error.value = cause?.message || '开票申请创建失败，请稍后重试'
  } finally {
    loading.value = false
  }
}
</script>
