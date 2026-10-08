# MyanKafe Platform integration

Finance accepts signed business events from MyanKafe Platform on `/api/v1/integrations/events`. The durable idempotency key is the integration connection plus `external_event_id`. The 24-hour HTTP idempotency middleware is not used for these events.

Platform does not choose account IDs. Mappings saved under Settings → Integrations decide the accounts, financial accounts, and sales channels. Missing mappings fail the event and create no journal. The same event can be retried after the mapping is saved.

Accepted events post immediately as `source_type = INTEGRATION` and `source_system = MYANKAFE_PLATFORM`. They do not use a staff login. Ordinary user transactions still require a real user.

The HMAC secret belongs in `INTEGRATION_SECRET_MYANKAFE_PLATFORM`. `integration_connections.secret_reference` only names that environment variable. The secret is not stored, logged, or returned by the API.

No production mapping is seeded by migration.
