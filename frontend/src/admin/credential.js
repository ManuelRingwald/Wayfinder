// ORCH-5b-2 (ADR 0012 §6; Firefly ADR 0023 / ADR 0024): the admin enters a source
// credential as two operator-facing fields — a client id and a client secret — but
// the secret store keeps a single opaque value per cred_ref. This module is the
// pure, testable join: the two fields are combined into one "client_id:client_secret"
// string before it is sealed and stored.
//
// The format mirrors Firefly's contract, which splits the resolved credential at
// the FIRST colon (client id = everything before it, client secret = the rest) and
// exchanges the pair via OAuth2 client-credentials for a bearer token (Firefly ADR
// 0024; OpenSky retired Basic auth). So a client secret may itself contain colons,
// but a client id must not — otherwise the split would misattribute part of the id
// to the secret. validateCredential enforces that (and non-empty fields) so the UI
// can block a save that would be silently misread downstream.

// validateCredential returns a German error message for an invalid client-id/
// client-secret pair, or '' when the pair is valid. Both fields are required; the
// client id must not contain a colon (the contract's first-colon split).
export function validateCredential(clientId, clientSecret) {
  const id = (clientId || '').trim()
  const secret = clientSecret || ''
  if (!id) return 'Client-ID fehlt.'
  if (!secret) return 'Client-Secret fehlt.'
  if (id.includes(':')) return 'Die Client-ID darf keinen Doppelpunkt enthalten.'
  return ''
}

// combineCredential joins a client id and client secret into the single
// "client_id:client_secret" value the secret store holds. The id is trimmed
// (leading/trailing whitespace in a client id is never significant); the secret is
// taken verbatim (it may legitimately contain spaces or colons). Returns null when
// the pair is invalid, so callers never store a malformed value.
export function combineCredential(clientId, clientSecret) {
  if (validateCredential(clientId, clientSecret) !== '') return null
  return `${(clientId || '').trim()}:${clientSecret || ''}`
}
