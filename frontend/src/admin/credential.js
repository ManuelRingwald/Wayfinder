// ORCH-5b-2 (ADR 0012 §6; Firefly ADR 0023): the admin enters a source
// credential as two operator-facing fields — a username and a password — but the
// secret store keeps a single opaque value per cred_ref. This module is the pure,
// testable join: the two fields are combined into one "user:pass" string before
// it is sealed and stored.
//
// The format mirrors Firefly's contract, which splits the resolved credential at
// the FIRST colon (username = everything before it, password = the rest). So a
// password may itself contain colons, but a username must not — otherwise the
// split would misattribute part of the username to the password. validateCredential
// enforces that (and non-empty fields) so the UI can block a save that would be
// silently misread downstream.

// validateCredential returns a German error message for an invalid username/
// password pair, or '' when the pair is valid. Both fields are required; the
// username must not contain a colon (the contract's first-colon split).
export function validateCredential(username, password) {
  const u = (username || '').trim()
  const p = password || ''
  if (!u) return 'Benutzername fehlt.'
  if (!p) return 'Passwort fehlt.'
  if (u.includes(':')) return 'Der Benutzername darf keinen Doppelpunkt enthalten.'
  return ''
}

// combineCredential joins a username and password into the single "user:pass"
// value the secret store holds. The username is trimmed (leading/trailing
// whitespace in a client id is never significant); the password is taken verbatim
// (a credential may legitimately contain spaces or colons). Returns null when the
// pair is invalid, so callers never store a malformed value.
export function combineCredential(username, password) {
  if (validateCredential(username, password) !== '') return null
  return `${(username || '').trim()}:${password || ''}`
}
