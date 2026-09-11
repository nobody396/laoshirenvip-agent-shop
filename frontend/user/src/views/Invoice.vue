<template>
  <div class="min-h-screen bg-background px-4 pb-16 pt-24 text-foreground">
    <div class="mx-auto max-w-3xl">
      <h1 class="text-3xl font-black">申请电子发票</h1>
	  <p class="mt-2 text-sm text-muted-foreground">填写资料并支付开票补款，支付成功后系统将开始处理</p>
	  <p class="mt-1 text-sm text-muted-foreground">开票项目：生产生活服务信息系统服务</p>

      <div v-if="result" class="mt-8 rounded-2xl border bg-card p-6 shadow-sm">
		<h2 class="text-xl font-bold">{{ result.status === 'pending_payment' ? '请扫码支付' : '系统正在处理' }}</h2>
		<p v-if="result.status !== 'pending_payment'" class="mt-2 text-sm text-emerald-600">发票开好后会自动发送到你填写的邮箱</p>
        <div class="mt-5 grid gap-6 md:grid-cols-2">
		  <img v-if="result.status === 'pending_payment' && qrImage" :src="qrImage" alt="支付宝付款二维码" class="mx-auto size-56 rounded-xl bg-white p-3" />
          <div class="space-y-3 text-sm">
            <p>申请编号：<strong>{{ result.request_no }}</strong></p>
            <p>订单实际结算金额：¥{{ result.original_amount }}</p>
            <p>发票金额（价税合计）：¥{{ result.invoice_total_amount }}</p>
            <p>开票服务费（3%）：¥{{ result.invoice_fee_amount }}</p>
			<p v-if="result.payment_method === 'wallet'">支付方式：钱包余额</p>
            <p>支付通道手续费<span v-if="Number(result.payment_fee_rate) > 0">（{{ result.payment_fee_rate }}%）</span>：¥{{ result.payment_fee_amount }}</p>
            <p class="text-lg">本次应付：<strong>¥{{ result.payment_amount }}</strong></p>
			<a v-if="result.status === 'pending_payment' && result.pay_url" :href="result.pay_url" target="_blank" rel="noopener noreferrer" class="inline-flex rounded-lg bg-primary px-4 py-2 font-semibold text-primary-foreground">打开支付宝付款</a>
          </div>
        </div>
      </div>

      <form v-else class="mt-8 space-y-6 rounded-2xl border bg-card p-6 shadow-sm" @submit.prevent="submit">
        <label class="grid gap-2 text-sm">
		  <span>{{ isRecharge ? '充值单号' : '订单号' }}</span>
          <Input v-model="form.order_no" required @input="clearPreview" @blur="loadPreview" />
        </label>
		<div v-if="isGMShop" class="grid gap-2 text-sm"><span>下单邮箱</span><Input v-model="guest.email" type="email" required @input="clearPreview" @blur="loadPreview" /></div>
        <div v-else-if="isGuest" class="grid gap-4 md:grid-cols-2">
          <label class="grid gap-2 text-sm"><span>下单邮箱</span><Input v-model="guest.email" type="email" required @input="clearPreview" @blur="loadPreview" /></label>
          <label class="grid gap-2 text-sm"><span>订单查询密码</span><Input v-model="guest.order_password" type="password" required @input="clearPreview" @blur="loadPreview" /></label>
        </div>

		<div v-if="previewLoading" class="rounded-xl border bg-muted/30 p-4 text-sm text-muted-foreground">正在计算开票费用…</div>
		<label v-if="preview || form.invoice_amount" class="grid gap-2 text-sm">
		  <span>发票金额（价税合计）</span>
		  <Input v-model="form.invoice_amount" type="number" inputmode="decimal" min="0.01" step="0.01" required @blur="loadPreview" />
		  <span class="text-xs text-muted-foreground">代顾客开票时，请填写你与顾客的实际成交金额，系统按该金额收取 3% 开票服务费</span>
		</label>
		<div v-if="preview && !previewLoading" class="rounded-xl border bg-muted/30 p-5">
		  <div class="font-semibold">开票费用预览</div>
		  <div class="mt-4 grid gap-3 text-sm md:grid-cols-2 lg:grid-cols-5">
			<div><div class="text-muted-foreground">订单实际结算金额</div><div class="mt-1 text-lg font-bold">¥{{ preview.order_amount }}</div></div>
			<div><div class="text-muted-foreground">发票金额（价税合计）</div><div class="mt-1 text-lg font-bold">¥{{ preview.invoice_total_amount }}</div></div>
			<div><div class="text-muted-foreground">开票服务费（3%）</div><div class="mt-1 text-lg font-bold">¥{{ preview.invoice_fee_amount }}</div></div>
			<div><div class="text-muted-foreground">支付通道手续费</div><div class="mt-1 text-lg font-bold">¥{{ preview.payment_fee_amount }}</div></div>
			<div><div class="text-muted-foreground">本次应付</div><div class="mt-1 text-lg font-bold text-primary">¥{{ preview.payment_amount }}</div></div>
		  </div>
		</div>
		<p v-if="previewError" class="text-sm text-destructive">{{ previewError }}</p>

		<div class="grid gap-4 md:grid-cols-2">
		  <div class="grid gap-2 text-sm"><span>发票类型</span><div class="flex h-11 items-center rounded-md border bg-muted/30 px-3 font-medium">普通发票（3%）</div></div>
		  <label class="grid gap-2 text-sm"><span>发票抬头</span><Input v-model="form.buyer_title" required /></label>
        </div>
        <label class="grid gap-2 text-sm"><span>统一社会信用代码</span><Input v-model="form.tax_number" required /></label>
        <label class="grid gap-2 text-sm"><span>发票接收邮箱</span><Input v-model="form.recipient_email" type="email" required /></label>

		<fieldset v-if="supportsWalletPayment" class="grid gap-3">
		  <legend class="text-sm">支付方式</legend>
		  <div class="grid gap-3 md:grid-cols-2">
			<label class="flex cursor-pointer items-center gap-3 rounded-xl border p-4" :class="form.payment_method === 'alipay' ? 'border-primary bg-primary/5' : ''">
			  <input v-model="form.payment_method" type="radio" value="alipay" @change="loadPreview" />
			  <span><span class="block font-medium">支付宝</span><span class="text-xs text-muted-foreground">扫码支付开票补款</span></span>
			</label>
			<label class="flex cursor-pointer items-center gap-3 rounded-xl border p-4" :class="form.payment_method === 'wallet' ? 'border-primary bg-primary/5' : ''">
			  <input v-model="form.payment_method" type="radio" value="wallet" @change="loadPreview" />
			  <span><span class="block font-medium">钱包</span><span class="text-xs text-muted-foreground">免新增支付通道手续费</span></span>
			</label>
		  </div>
		</fieldset>
		<div v-else class="grid gap-2 text-sm"><span>支付方式</span><div class="flex h-11 items-center rounded-md border bg-muted/30 px-3">支付宝扫码支付</div></div>

        <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
        <Button type="submit" class="w-full" :disabled="loading || previewLoading">{{ loading ? '正在提交…' : '确认资料并支付' }}</Button>
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
const supportsWalletPayment = computed(() => !isGuest.value && !isGMShop.value)
const loading = ref(false)
const error = ref('')
const result = ref<any>(null)
const preview = ref<any>(null)
const previewLoading = ref(false)
const previewError = ref('')
const qrImage = ref('')
const guest = reactive({ email: '', order_password: '' })
const form = reactive({
	order_no: String(route.query.order_no || route.query.recharge_no || ''), invoice_type: 'ordinary', buyer_title: '', tax_number: '',
	recipient_email: '', invoice_amount: '', payment_method: 'wallet',
})
let statusTimer: number | undefined

function clearPreview() {
  preview.value = null
  previewError.value = ''
  form.invoice_amount = ''
}

async function loadPreview() {
  const orderNo = form.order_no.trim()
  if (!orderNo || (isGMShop.value && !guest.email.trim()) || (isGuest.value && (!guest.email.trim() || !guest.order_password))) {
    preview.value = null
    previewError.value = ''
    return
  }
  previewLoading.value = true
  previewError.value = ''
  try {
    let response
    const paymentMethod = supportsWalletPayment.value ? form.payment_method : 'alipay'
    const payload = { order_no: orderNo, invoice_amount: form.invoice_amount, payment_method: paymentMethod }
    if (isGMShop.value) response = await invoiceAPI.previewGMShop({ ...payload, order_email: guest.email })
    else if (isRecharge.value) response = await invoiceAPI.previewRecharge(payload)
    else if (isGuest.value) response = await invoiceAPI.previewGuest({ ...payload, ...guest })
    else response = await invoiceAPI.preview(payload)
    preview.value = response.data.data
    if (!form.invoice_amount) form.invoice_amount = preview.value.invoice_total_amount
  } catch (cause: any) {
    preview.value = null
    previewError.value = cause?.message || '暂时无法预览发票金额'
  } finally {
    previewLoading.value = false
  }
}

async function loadRequest(requestNo: string) {
  try {
    result.value = (await invoiceAPI.get(requestNo)).data.data
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
  if (requestNo) {
    void loadRequest(requestNo)
    statusTimer = window.setInterval(() => void loadRequest(requestNo), 3000)
    return
  }
  if (form.order_no) void loadPreview()
})
onBeforeUnmount(() => { if (statusTimer) window.clearInterval(statusTimer) })

async function submit() {
  error.value = ''
  await loadPreview()
  if (!preview.value) return
  loading.value = true
  try {
	let response
	const payload = { ...form, payment_method: supportsWalletPayment.value ? form.payment_method : 'alipay' }
	if (isGMShop.value) response = await invoiceAPI.createGMShop({ ...payload, order_email: guest.email })
	else if (isRecharge.value) response = await invoiceAPI.createRecharge(payload)
	else if (isGuest.value) response = await invoiceAPI.createGuest({ ...payload, ...guest })
	else response = await invoiceAPI.create(payload)
	result.value = response.data.data
    const qr = result.value.qr_code || result.value.pay_url
    if (qr) qrImage.value = await QRCode.toDataURL(qr, { width: 360, margin: 1 })
  } catch (cause: any) {
    error.value = cause?.message || '开票申请创建失败，请稍后重试'
  } finally {
    loading.value = false
  }
}
</script>
