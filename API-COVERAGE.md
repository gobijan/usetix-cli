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
| Shared tasks | `tasks list/show/people/create/update/move/comments/comment/archive/restore/delete` | `api ... /admin/tasks...`, `/admin/task_assignees`, `/admin/task_uploads...` | Typed for board filters, assignments, due dates, rich text, comments, ordering, versions and archives; private upload metadata/downloads are direct, storage PUT is an external HTTP step |
| Events | `events list/show/open-answers/create/update/delete/publish/unpublish` | `api ... /admin/events...` | Typed for event lifecycle, post-purchase attendee notes, and missing-answer follow-up; fee policy and image attachments are direct |
| Admission and arrivals | `events arrivals SLUG` | `api GET /admin/events/:slug/arrivals` | Typed totals, redemption rate, peak and chronological intervals; read-only, including assigned events for co-organizers |
| Venues | — | `api ... /admin/venues...` | Direct |
| Performers | — | `api ... /admin/performers...` | Direct |
| Tickets | — | `api ... /admin/events/:slug/...tickets...` | Direct |
| Ticket upgrades | `events upgrades list/offer/update/pause/resume/remove`, `ticket-upgrades status/activate/deactivate` | `api ... /admin/events/:slug/ticket_upgrades...`, `/admin/account_settings/ticket_upgrade_activation` | Typed; see [Ticket upgrades](#ticket-upgrades) |
| Event FAQs | — | `api ... /admin/events/:slug/faq_items...` | Direct |
| Custom checkout fields | — | `api ... /admin/events/:slug/custom_fields...` | Direct |
| Promo codes | `promo-codes list/show/create/update/deactivate/reactivate` | `api ... /admin/promo_codes...` | Typed codes, zero discounts, share links, limits and promoter assignments; deletion remains direct |
| Promoter reports | `promoters list` | `api GET /admin/promoters` | Typed per-promoter/code sales and refund-adjusted revenue, with event and purchase-period filters; organizer only |
| Gift vouchers | `vouchers list/report/show/issue/adjust/block/unblock/retry-delivery/import`, `vouchers products list/show/create/update/archive/remove-image` | `api ... /admin/vouchers...`, `/admin/voucher_deliveries/:id/retry`, `/admin/voucher_lookup`, `/admin/voucher_products...`, `/admin/voucher_imports...` | Typed for cursor pagination, bonus-priced products, common balance operations, delivery inspection/retry, product lifecycle/artwork removal, body-safe exact-code lookup, reporting, and atomic CSV imports; direct for CSV export and catalog reordering |
| Guest list and seat moves | — | `api ... /admin/events/:slug/guest_...` | Direct |
| Seat map and holds | `events occupancy`, `events holds list/show/create/update/add-places/remove-places/send-link/release/release-places` | `api ... /admin/events/:slug/occupancy`, `/holds...`, `/place_release` | Typed; see [Seat map and holds](#seat-map-and-holds) |
| Guest-list signup links and review | `events guest-list forms/create/form/configure/rotate/requests/approve/reject` | `api ... /admin/events/:slug/guest_list_forms`, `/guest_list_forms/:public_id/rotation`, `/guest_requests...` | Multiple active-by-default links, exact link selection, confirmed URL rotation, manual/automatic approval, asked details (company, phone and companions' names as hidden/optional/required, order-level checkout questions), numeric request pagination, confirmed review including companions added after approval; existing parties and seat moves remain direct |
| Orders | `orders list/show/refund/cancel/archive/unarchive` | `api ... /admin/orders...` | Fully typed, including customer order links (`shop_url`), mixed product lines, ticket admissions, where box-office orders were paid (`outside_payment`, `sales_point`), refunds, booking cancellation, and archival |
| Booking tickets (box office) | `events book` | `api POST /admin/events/:slug/bookings` | Typed for free guest-list bookings and bookings paid outside Usetix, by ticket quantities or by places, tables and standing areas; see [Booking tickets](#booking-tickets) |
| Customers | `customers list/show/update`, `customers contacts list/show/log/update/delete` | `api ... /admin/customers...` | Typed for the cursor-paginated directory with event, search, period and marketing filters, details with orders, correcting salutation/title/name/company/phone, and interaction timelines; CSV/XLSX export remains direct |
| Analytics and Live View | `analytics shares/share/revoke` for expiring report links | `api GET /admin/analytics...` | Report sharing is typed; full historical and Live View data remain direct |
| Analytics settings | — | `api ... /admin/account_settings/analytics` | Direct; controls collection and consent-gated external tracking identifiers |
| Advertising (limited beta) | — | `api GET /admin/advertising` | Direct for enabled accounts; Meta connection summary and cursor-paginated campaigns |
| CSV/XLSX/PDF exports | `sales-points export` | `api GET ...csv --output FILE` | Direct via `--output` (orders, customers, attendees, analytics); a `GET` of an export authenticates with the API token, and an HTML page answers a token with `401` |
| Shop settings | — | `api ... /admin/account_settings/shop` | Direct; exposes `shop_url` for the public feed host |
| Checkout settings | — | `api ... /admin/account_settings/checkout` | Direct; buyer fields, ticket price display, and account fee defaults |
| Checkout fees (compatibility) | — | `api ... /admin/account_settings/payments` | Direct; retained for existing clients |
| Memberships and invitations | `team list/invite/access/deactivate/reactivate`, `team invitations resend/revoke`, `events invite-co-organizer` | `api ... /admin/memberships`, `/admin/invitations` | Team roles, complete event assignments, invitations and revocation, sales point members named by their sales point; venue only |
| Sales points | `sales-points list/show/create/update/archive/invite/settle/settlements/unsettle/export` | `api ... /admin/sales_points...` | Typed; see [Sales points](#sales-points) |
| Event duplication | `events duplicate SLUG` | `api POST /admin/events/SLUG/duplication` | Draft copy; co-organizers receive access to their copy |
| Event media and gallery | — | `api PATCH /admin/events/SLUG` | Signed upload IDs, gallery order/removal, documents and their shop names (`document_titles`, `new_document_titles`, and the private equivalents), video and artwork; same editor validations |
| Scanner | — | `api ... /scanner/...` | Direct |
| Public event feed | — | `api GET /events --no-auth --api-url SHOP_URL` | Direct; lives on the shop host (`shop_url` from shop settings), not on `app.usetix.io` |
| Active Storage direct-upload metadata | — | `api POST /rails/active_storage/direct_uploads ...` | Direct; uploading bytes to the returned storage URL remains an external HTTP step |

## Dashboard-only operations

These exist in the product but have no JSON endpoints yet, so neither typed
nor direct access can reach them: outbound webhook management, API token
lifecycle, seat-map
editing, scanner device pairing, walk-in sales, and billing/connect settings.
If one of these becomes a real CLI need, the JSON endpoint belongs in the
Rails application first.

## Typed-command order

The next native slices should follow actual organizer frequency, not mirror the
API mechanically:

1. ~~Event show/create/update/publication.~~ Done.
2. ~~Orders read and refund workflows.~~ Done.
3. ~~Customer list/show/update workflows.~~ Done.
4. Tickets and manual guest-list parties/seat moves. Signup links, request reviews, gift vouchers, promo codes, and promoter reports are already typed.
5. Venues, performers, analytics, and account settings.
6. Scanner workflows if terminal scanning proves useful alongside the native
   scanner app.

Every typed command must retain `--json` stability, account scoping, explicit
confirmation for destructive actions, and a direct mapping to an existing
documented endpoint.

Personal co-organizer tokens use the same authentication and event commands. They follow current membership/event access and cannot call account-wide team, scanner, refund or settings endpoints.

## Lounge reservations

Typed commands: `events lounges list`, `bookings`, `configure`, `accept`, `reject`, and `cancel`.
Reviews require `--yes`; acceptance charges Credit immediately. The fee preview uses the current account price.
Venue catalog CRUD, images and per-event price overrides use `usetix api` with the documented venue/event routes.
See https://www.usetix.io/docs/api/lounges/. Lounge bookings do not issue admission tickets.

Booking questions are configured in the dashboard.

## Ticket upgrades

Typed commands: `events upgrades list`, `offer`, `update`, `pause`, `resume` and
`remove` for an event's upgrades, and `ticket-upgrades status`, `activate` and
`deactivate` for the account-wide activation.
`activate` without `--yes` reads the activation and shows the account's fee per
switched ticket and whether Usetix Credit covers it; `--yes` activates and
agrees to the fee. `deactivate` and `remove` also require `--yes`; removing an
upgrade deletes its results. Personal co-organizer tokens manage their assigned
events' upgrades but cannot change the activation (`403`).
See https://www.usetix.io/docs/api/ticket-upgrades/.

## Seat map and holds

Typed commands: `events occupancy` for the seat map's counts, open holds,
standing areas and, with `--state`, the places in one state, and
`events holds list`, `show`, `create`, `update`, `add-places`,
`remove-places`, `send-link`, `release` and `release-places`.
`holds list` lists the open holds, or by `--status` (`fulfilled`, `released`,
`expired`, `all`). `send-link` without `--yes` reads the hold and names the
recipient; `--yes` emails the buy link. `release`, `remove-places` and
`release-places` put places back on sale and require `--yes`. The CLI reads
the occupancy afresh each time and sends no `If-None-Match` or
`Idempotency-Key` header. See https://www.usetix.io/docs/api/holds/.

## Booking tickets

Typed command: `events book SLUG` with `--payment` (`complimentary`, `till`,
`bank_transfer`, `invoice` or `other`), `--ticket TICKET_ID=QUANTITY` for
events without a seat map, or `--place`/`--table` with
`--category CATEGORY_ID=TICKET_ID` and `--standing-area` with
`--standing-ticket` and `--quantity` for seated events. Free bookings need
`--name` and one kind of selection with one ticket, never whole tables; the
CLI checks that before asking. Booking always requires `--yes`; without it the command says what would happen and books
nothing. Paid bookings take Usetix's fee from Usetix Credit, which the API
does not preview; missing credit returns `422` naming the fee. Personal
co-organizer tokens get `403` for paid bookings. The CLI sends no
`Idempotency-Key`. See
https://www.usetix.io/docs/api/orders/#post-admineventsevent_slugbookings.

## Sales points

Typed commands: `sales-points list`, `show`, `create`, `update`, `archive`,
`invite`, `settle`, `settlements`, `unsettle` and `export`. `--event` takes
event slugs and sends their numeric IDs. `settle` without `--yes` reads the
sales point and shows the open period it would settle; `archive`, `settle` and
`unsettle` require `--yes`. `settlements` prints the latest 24 settlements
from the sales point's `show` response under `settlements`. `export` saves
`GET /admin/sales_points/:id.csv` with the API token and refuses any answer
that isn't `text/csv`. Sales points need a venue token; personal
co-organizer tokens and Usetix support get `403`. See
https://www.usetix.io/docs/api/sales-points/.
