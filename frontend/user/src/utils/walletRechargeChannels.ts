export const WALLET_RECHARGE_CHANNEL_DISCOVERY_AMOUNT = '1.00'

export const resolveWalletRechargeChannelRequestAmount = (
  rawAmount: unknown,
  amountCents: number | null,
): string => {
  const amount = String(rawAmount ?? '').trim()
  if (amount && amountCents !== null && amountCents > 0) return amount
  return WALLET_RECHARGE_CHANNEL_DISCOVERY_AMOUNT
}
