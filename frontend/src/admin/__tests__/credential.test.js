import { describe, it, expect } from 'vitest'
import { validateCredential, combineCredential } from '@/admin/credential.js'

describe('admin credential — validateCredential', () => {
  it('accepts a normal client-id/client-secret pair', () => {
    expect(validateCredential('client-abc', 's3cr3t')).toBe('')
  })

  it('requires a client id', () => {
    expect(validateCredential('', 'pw')).not.toBe('')
    expect(validateCredential('   ', 'pw')).not.toBe('')
  })

  it('requires a client secret', () => {
    expect(validateCredential('client-abc', '')).not.toBe('')
  })

  it('rejects a colon in the client id (first-colon split would misread it)', () => {
    expect(validateCredential('cli:ent', 'pw')).not.toBe('')
  })

  it('allows a colon in the client secret (everything after the first colon)', () => {
    expect(validateCredential('client-abc', 'pa:ss:word')).toBe('')
  })
})

describe('admin credential — combineCredential', () => {
  it('joins client id and secret as client_id:client_secret', () => {
    expect(combineCredential('client-abc', 's3cr3t')).toBe('client-abc:s3cr3t')
  })

  it('trims the client id but keeps the secret verbatim', () => {
    expect(combineCredential('  client-abc  ', '  s3cr3t  ')).toBe('client-abc:  s3cr3t  ')
  })

  it('preserves colons in the client secret', () => {
    expect(combineCredential('client-abc', 'pa:ss')).toBe('client-abc:pa:ss')
  })

  it('returns null for an invalid pair', () => {
    expect(combineCredential('', 'pw')).toBeNull()
    expect(combineCredential('client-abc', '')).toBeNull()
    expect(combineCredential('cli:ent', 'pw')).toBeNull()
  })
})
