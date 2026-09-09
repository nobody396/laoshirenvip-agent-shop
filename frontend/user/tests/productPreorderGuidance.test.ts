import assert from 'node:assert/strict'
import test from 'node:test'

import {
  isChatGptMembershipProduct,
  resolveChatGptPlanGuidance,
} from '../src/utils/productPreorderGuidance.ts'

test('shows preorder guidance only for the grouped ChatGPT membership product', () => {
  assert.equal(isChatGptMembershipProduct({
    title: {
      'zh-CN': 'ChatGPT会员充值',
      'en-US': 'ChatGPT Membership Recharge',
    },
  }), true)
  assert.equal(isChatGptMembershipProduct({ title: { 'zh-CN': 'ChatGPT Go 1个月' } }), false)
  assert.equal(isChatGptMembershipProduct({ title: { 'zh-CN': 'Claude会员充值' } }), false)
})

test('distinguishes expired-only Philippines plans from renewable iOS and Go plans', () => {
  assert.equal(resolveChatGptPlanGuidance('ChatGPT Plus 菲区 1个月'), 'expired-only')
  assert.equal(resolveChatGptPlanGuidance('ChatGPT Pro 20X Philippines — 1 Month'), 'expired-only')
  assert.equal(resolveChatGptPlanGuidance('ChatGPT Plus iOS 1个月'), 'renewable')
  assert.equal(resolveChatGptPlanGuidance('ChatGPT Go 1个月'), 'renewable')
  assert.equal(resolveChatGptPlanGuidance(''), 'compare')
})
