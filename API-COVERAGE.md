# API coverage

Usetix uses the existing dashboard and scanner URLs with JSON content
negotiation. The CLI does not introduce `/api/v1` or duplicate server-side
business logic.

There are two coverage levels:

- **Typed** — a discoverable Cobra command with domain-aware flags and styled
  output.
- **Direct** — complete request/response access through
  `usetix api METHOD PATH`, including JSON from `--data`, `@file`, or stdin,
  and raw downloads via `--output` for CSV, XLSX, and PDF exports.

Direct access means the documented JSON API is functionally reachable today.
Typed coverage will grow where it materially improves daily use.

| Documented area | Typed command | Direct access | Status |
|---|---|---|---|
| Authentication | `auth login`, `auth status`, `auth logout` | n/a | Local token validation and storage are typed; server token creation/revocation remains in Settings |
| Events | `events list/show/create/update/delete/publish/unpublish` | `api ... /admin/events...` | Fully typed; fee policy and image attachments are direct |
| Venues | — | `api ... /admin/venues...` | Direct |
| Performers | — | `api ... /admin/performers...` | Direct |
| Tickets | — | `api ... /admin/events/:slug/...tickets...` | Direct |
| Event FAQs | — | `api ... /admin/events/:slug/faq_items...` | Direct |
| Custom checkout fields | — | `api ... /admin/events/:slug/custom_fields...` | Direct |
| Promo codes | — | `api ... /admin/promo_codes...` | Direct |
| Guest list and seat moves | — | `api ... /admin/events/:slug/guest_...` | Direct |
| Orders | `orders list/show/refund/cancel/archive/unarchive` | `api ... /admin/orders...` | Fully typed, including refunds, booking cancellation, and archival |
| Customers | — | `api ... /admin/customers...` | Direct |
| Analytics and Live View | — | `api GET /admin/analytics...` | Direct |
| CSV/XLSX/PDF exports | — | `api GET ...csv --output FILE` | Direct via `--output` (orders, customers, attendees, analytics) |
| Shop settings | — | `api ... /admin/account_settings/shop` | Direct; exposes `shop_url` for the public feed host |
| Checkout fees | — | `api ... /admin/account_settings/payments` | Direct |
| Memberships | — | `api GET /admin/memberships` | Direct read (active, deactivated, pending invitations); invitation and role mutations are dashboard-only today |
| Scanner | — | `api ... /scanner/...` | Direct |
| Public event feed | — | `api GET /events --no-auth --api-url SHOP_URL` | Direct; lives on the shop host (`shop_url` from shop settings), not on `app.usetix.io` |
| Active Storage direct-upload metadata | — | `api POST /rails/active_storage/direct_uploads ...` | Direct; uploading bytes to the returned storage URL remains an external HTTP step |

## Dashboard-only operations

These exist in the product but have no JSON endpoints yet, so neither typed
nor direct access can reach them: outbound webhook management, API token
lifecycle, team invitation and role mutations, event duplication, seat-map
editing, scanner device pairing, walk-in sales, and billing/connect settings.
If one of these becomes a real CLI need, the JSON endpoint belongs in the
Rails application first.

## Typed-command order

The next native slices should follow actual organizer frequency, not mirror the
API mechanically:

1. ~~Event show/create/update/publication.~~ Done.
2. ~~Orders read and refund workflows.~~ Done.
3. Customers read workflows.
4. Tickets, promo codes, and guest-list operations.
5. Venues, performers, analytics, and account settings.
6. Scanner workflows if terminal scanning proves useful alongside the native
   scanner app.

Every typed command must retain `--json` stability, account scoping, explicit
confirmation for destructive actions, and a direct mapping to an existing
documented endpoint.
