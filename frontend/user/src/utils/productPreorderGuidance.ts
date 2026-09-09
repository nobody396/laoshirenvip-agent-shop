export type ChatGptPlanGuidance = 'renewable' | 'expired-only' | 'compare'

const localizedTextValues = (value: unknown): string => {
  if (typeof value === 'string') return value
  if (!value || typeof value !== 'object') return ''
  return Object.values(value as Record<string, unknown>)
    .filter((item): item is string => typeof item === 'string')
    .join(' ')
}

export const isChatGptMembershipProduct = (product: any): boolean => {
  const title = localizedTextValues(product?.title).toLowerCase()
  if (!title.includes('chatgpt')) return false

  return (
    title.includes('会员充值') ||
    title.includes('會員充值') ||
    title.includes('membership recharge')
  )
}

export const resolveChatGptPlanGuidance = (skuLabel: string): ChatGptPlanGuidance => {
  const label = String(skuLabel || '').toLowerCase()
  if (!label) return 'compare'

  if (label.includes('菲区') || label.includes('菲區') || label.includes('philippin')) {
    return 'expired-only'
  }
  if (label.includes('ios') || /chatgpt\s+go\b/.test(label)) {
    return 'renewable'
  }
  return 'compare'
}
