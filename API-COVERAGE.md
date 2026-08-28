# API coverage

Usetix uses the existing dashboard and scanner URLs with JSON content
negotiation. The CLI does not introduce `/api/v1` or duplicate server-side
business logic.

There are two coverage levels:

- **Typed** — a discoverable Cobra command with domain-aware flags and styled
  output.
- **Direct** — complete request/response access through
  `usetix api METHOD PATH`, including JSON from `--data`, `@file`, or stdin.

Direct access means the documented JSON API is functionally reachable today.
Typed coverage will grow where it materially improves daily use.

| Documented area | Typed command | Direct access | Status |
|---|---|---|---|
| Authentication | `auth login`, `auth status`, `auth logout` | n/a | Local token validation and storage are typed; server token creation/revocation remains in Settings |
| Events | `events list` | `api ... /admin/events...` | List is typed; show, create, update, delete, and publication are direct |
| Venues | — | `api ... /admin/venues...` | Direct |
| Performers | — | `api ... /admin/performers...` | Direct |
| Tickets | — | `api ... /admin/events/:slug/...tickets...` | Direct |
| Event FAQs | — | `api ... /admin/events/:slug/faq_items...` | Direct |
| Custom checkout fields | — | `api ... /admin/events/:slug/custom_fields...` | Direct |
| Promo codes | — | `api ... /admin/promo_codes...` | Direct |
| Guest list and seat moves | — | `api ... /admin/events/:slug/guest_...` | Direct |
| Orders | — | `api ... /admin/orders...` | Direct |
| Customers | — | `api ... /admin/customers...` | Direct |
| Analytics and Live View | — | `api GET /admin/analytics...` | Direct |
| Shop settings | — | `api ... /admin/account_settings/shop` | Direct |
| Checkout fees | — | `api ... /admin/account_settings/payments` | Direct |
| Memberships and invitations | — | `api GET /admin/memberships` | Direct |
| Scanner | — | `api ... /scanner/...` | Direct |
| Public event feed | — | `api GET /events... --no-auth` | Direct |
| Active Storage direct-upload metadata | — | `api POST /rails/active_storage/direct_uploads ...` | Direct; uploading bytes to the returned storage URL remains an external HTTP step |

## Typed-command order

The next native slices should follow actual organizer frequency, not mirror the
API mechanically:

1. Event show/create/update/publication.
2. Orders and customers read workflows.
3. Tickets, promo codes, and guest-list operations.
4. Venues, performers, analytics, and account settings.
5. Scanner workflows if terminal scanning proves useful alongside the native
   scanner app.

Every typed command must retain `--json` stability, account scoping, explicit
confirmation for destructive actions, and a direct mapping to an existing
documented endpoint.
