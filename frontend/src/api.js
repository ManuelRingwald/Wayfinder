// apiFetch wraps fetch for the Wayfinder JSON APIs: it always sends/expects JSON
// and normalises the result into { ok, status, data, error } so callers never
// juggle response parsing or status branching. A non-2xx with an {"error": "..."}
// body surfaces that message; otherwise a generic "HTTP <status>" is used.
//
// Shared by the admin dashboard store and the ASD session store so both speak to
// the backend the same way (same-origin cookies carry the session).
export async function apiFetch(path, options = {}) {
  let res
  try {
    res = await fetch(path, {
      headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
      ...options,
    })
  } catch (e) {
    return { ok: false, status: 0, data: null, error: `network error: ${e?.message ?? e}` }
  }
  let data = null
  const text = await res.text()
  if (text) {
    try { data = JSON.parse(text) } catch { data = null }
  }
  if (!res.ok) {
    return { ok: false, status: res.status, data, error: (data && data.error) || `HTTP ${res.status}` }
  }
  return { ok: true, status: res.status, data, error: null }
}
