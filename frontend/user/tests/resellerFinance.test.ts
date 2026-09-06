import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import {
  getResellerFinanceStatusView,
  getResellerLedgerTypeKey,
  getResellerWithdrawDisabledReasonKey,
  isResellerWithdrawEnabled,
} from '../src/utils/resellerFinance.ts'

test('reseller withdraw availability follows dashboard contract', () => {
  assert.equal(isResellerWithdrawEnabled({ withdraw_enabled: true }), true)
  assert.equal(isResellerWithdrawEnabled({ withdraw_enabled: false }), false)
  assert.equal(isResellerWithdrawEnabled(null), false)
})

test('reseller withdraw disabled reason maps backend reason keys', () => {
  assert.equal(getResellerWithdrawDisabledReasonKey('profile_inactive'), 'profileInactive')
  assert.equal(getResellerWithdrawDisabledReasonKey('settlement_unavailable'), 'settlementUnavailable')
  assert.equal(getResellerWithdrawDisabledReasonKey('unexpected'), 'default')
  assert.equal(getResellerWithdrawDisabledReasonKey(undefined), 'default')
})

test('reseller finance status prioritizes inactive profile before settlement status', () => {
  assert.deepEqual(
    getResellerFinanceStatusView(null),
    { namespace: 'profileStatusMap', key: 'unknown', badgeTone: 'neutral' },
  )
  assert.deepEqual(
    getResellerFinanceStatusView({ status: 'active', settlement_status: 'normal' }),
    { namespace: 'settlementStatusMap', key: 'normal', badgeTone: 'success' },
  )
  assert.deepEqual(
    getResellerFinanceStatusView({ status: 'active', settlement_status: 'frozen' }),
    { namespace: 'settlementStatusMap', key: 'frozen', badgeTone: 'warning' },
  )
  assert.deepEqual(
    getResellerFinanceStatusView({ status: 'disabled', settlement_status: 'normal' }),
    { namespace: 'profileStatusMap', key: 'disabled', badgeTone: 'neutral' },
  )
})

test('reseller ledger type mapping includes all backend ledger types', () => {
  assert.equal(getResellerLedgerTypeKey('order_profit'), 'orderProfit')
  assert.equal(getResellerLedgerTypeKey('refund_deduct'), 'refundDeduct')
  assert.equal(getResellerLedgerTypeKey('withdraw_lock'), 'withdrawLock')
  assert.equal(getResellerLedgerTypeKey('manual_adjust'), 'manualAdjust')
  assert.equal(getResellerLedgerTypeKey('withdraw_paid'), 'withdrawPaid')
  assert.equal(getResellerLedgerTypeKey('other'), null)
})

test('reseller withdrawal saves one Alipay account and reuses it', async () => {
  const view = await readFile(new URL('../src/views/reseller/ResellerWithdraws.vue', import.meta.url), 'utf8')
  const api = await readFile(new URL('../src/api/reseller.ts', import.meta.url), 'utf8')
  assert.match(view, /payoutAlipayAccount/)
  assert.match(view, /savedPayoutAlipayAccount/)
  assert.match(view, /resellerConsole\.withdraws\.alipay/)
  assert.doesNotMatch(view, /form\.channel/)
  assert.doesNotMatch(view, /form\.account/)
  assert.match(api, /payout-alipay-account/)
})
