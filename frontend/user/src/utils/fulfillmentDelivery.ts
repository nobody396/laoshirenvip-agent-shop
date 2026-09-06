export const fulfillmentDeliveryLineURL = (line?: string): string => {
  const match = String(line || '').match(/https?:\/\/[^\s<>"']+/i)
  if (!match) return ''
  const candidate = match[0].replace(/[),.;，。；）】]+$/u, '')
  try {
    const parsed = new URL(candidate)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:' ? candidate : ''
  } catch {
    return ''
  }
}
