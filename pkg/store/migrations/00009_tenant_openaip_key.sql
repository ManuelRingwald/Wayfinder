-- +migrate up
-- ONB-6 (ADR 0011, F3): OpenAIP per tenant. Until now the OpenAIP API key was a
-- single process-global value (WAYFINDER_OPENAIP_API_KEY) and one cached region
-- served every tenant — a multi-tenant isolation leak (one customer's key/quota
-- and cached airspace shown to all) and wrong for tenants watching different
-- regions. This column holds an optional per-tenant key. NULL means "no own key":
-- the tenant falls back to the global key (backward compatible). The key is a
-- secret; it is never read into the shared Tenant row, only via the dedicated
-- Get/SetOpenAIPKey accessors, and is never returned to the browser.
ALTER TABLE tenants ADD COLUMN openaip_api_key TEXT;

-- +migrate down
ALTER TABLE tenants DROP COLUMN IF EXISTS openaip_api_key;
