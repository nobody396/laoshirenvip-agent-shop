import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import {
  WALLET_RECHARGE_CHANNEL_DISCOVERY_AMOUNT,
  resolveWalletRechargeChannelRequestAmount,
} from '../src/utils/walletRechargeChannels.ts'

test('wallet recharge discovers and preselects channels before an amount is entered', async () => {
  assert.equal(WALLET_RECHARGE_CHANNEL_DISCOVERY_AMOUNT, '1.00')
  assert.equal(resolveWalletRechargeChannelRequestAmount('', null), '1.00')
  assert.equal(resolveWalletRechargeChannelRequestAmount('not-money', null), '1.00')
  assert.equal(resolveWalletRechargeChannelRequestAmount(' 25.50 ', 2550), '25.50')

  const panel = await readFile(new URL('../src/views/personal/WalletPanel.vue', import.meta.url), 'utf8')
  assert.match(panel, /resolveWalletRechargeChannelRequestAmount/)
  assert.match(panel, /await appStore\.loadConfig\(\)/)
  assert.match(panel, /scheduleLoadPaymentChannels\(\)/)
  assert.match(panel, /rechargeForm\.channelId = first\.id/)
})
