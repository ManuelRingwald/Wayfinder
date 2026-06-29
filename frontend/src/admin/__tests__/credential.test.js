import { describe, it, expect } from 'vitest'
import { validateCredential, combineCredential } from '@/admin/credential.js'

describe('admin credential — validateCredential', () => {
  it('accepts a normal username/password pair', () => {
    expect(validateCredential('alice', 's3cr3t')).toBe('')
  })

  it('requires a username', () => {
    expect(validateCredential('', 'pw')).not.toBe('')
    expect(validateCredential('   ', 'pw')).not.toBe('')
  })

  it('requires a password', () => {
    expect(validateCredential('alice', '')).not.toBe('')
  })

  it('rejects a colon in the username (first-colon split would misread it)', () => {
    expect(validateCredential('al:ice', 'pw')).not.toBe('')
  })

  it('allows a colon in the password (everything after the first colon)', () => {
    expect(validateCredential('alice', 'pa:ss:word')).toBe('')
  })
})

describe('admin credential — combineCredential', () => {
  it('joins username and password as user:pass', () => {
    expect(combineCredential('alice', 's3cr3t')).toBe('alice:s3cr3t')
  })

  it('trims the username but keeps the password verbatim', () => {
    expect(combineCredential('  alice  ', '  s3cr3t  ')).toBe('alice:  s3cr3t  ')
  })

  it('preserves colons in the password', () => {
    expect(combineCredential('alice', 'pa:ss')).toBe('alice:pa:ss')
  })

  it('returns null for an invalid pair', () => {
    expect(combineCredential('', 'pw')).toBeNull()
    expect(combineCredential('alice', '')).toBeNull()
    expect(combineCredential('al:ice', 'pw')).toBeNull()
  })
})
