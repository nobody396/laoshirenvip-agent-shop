export const inviteCodeFromRegistrationHash = (rawHash: unknown) => {
  const hash = String(rawHash ?? '').trim()
  const prefix = '#invite='
  if (!hash.startsWith(prefix)) return ''
  const encoded = hash.slice(prefix.length)
  if (!encoded || encoded.length > 1024) return ''
  try {
    return decodeURIComponent(encoded).trim().slice(0, 512)
  } catch {
    return ''
  }
}
