# User Ticket Center and Admin Conversation Workflow

Date: 2026-08-31 (Asia/Shanghai)

## Objective

Add a private, multi-turn support ticket workflow to the existing YuAPI
dashboard. Users can report issues or submit a manual refund request, and
administrators can review and reply from the admin dashboard until the issue is
resolved.

The refund category is communication only. The system never credits a balance,
creates a refund transaction, or changes billing records automatically.

## Scope and invariants

- User-facing ticket center: list, status/type filters, create ticket, view
  conversation, reply, and upload attachments.
- Admin ticket center: list/search/filter, view any ticket, reply, change
  priority, close, and reopen.
- Every message is append-only and records the author, role snapshot, body, and
  creation time. The full conversation remains auditable.
- Users can access only tickets they own. Administrators use the existing
  `AdminAuth` permission boundary.
- Closed tickets are read-only for users. An administrator may reopen them.
- No email, SMS, or external notification service is added in the first
  version. In-app unread state is sufficient.
- Existing usage logs, top-ups, balances, billing, and transaction records are
  not modified by ticket operations.

## Data model

Add three GORM models and register them with the existing cross-database
`AutoMigrate` path:

### Ticket

- `id` primary key;
- `user_id` indexed;
- `subject` bounded text;
- `category`: `general` or `refund`;
- `status`: `open`, `pending_user`, `pending_admin`, or `closed`;
- `priority`: `normal`, `high`, or `urgent`;
- `message_count`, `unread_for_user`, and `unread_for_admin` counters;
- `created_at` and `updated_at` timestamps;
- `last_message_at` indexed for sorting.

### TicketMessage

- `id` primary key;
- `ticket_id` indexed;
- `author_id` indexed;
- `author_role` snapshot (`user`, `admin`);
- `body` bounded text;
- `created_at` indexed;
- no edit or delete operation in the initial version.

### TicketAttachment

- `id` primary key;
- `ticket_id` and `message_id` indexed;
- `uploader_id` indexed;
- private storage key and sanitized display name;
- MIME type and byte size;
- `created_at`.

Foreign-key behavior should follow current project conventions and remain
compatible with SQLite, MySQL, and PostgreSQL. Deleting a ticket is not exposed
to ordinary users; an admin archival policy can be added later without
destroying the conversation by default.

## API contract

All endpoints use the existing JSON response wrapper and auth middleware.

User endpoints under `/api/tickets`:

- `GET /api/tickets`: paginated own-ticket list with filters;
- `POST /api/tickets`: create a ticket and its first message atomically;
- `GET /api/tickets/:id`: own-ticket detail and ordered messages;
- `POST /api/tickets/:id/messages`: append a user reply atomically;
- `POST /api/tickets/:id/attachments`: upload a private attachment for a new
  message;
- `GET /api/tickets/:id/attachments/:attachment_id`: authorized download.

Admin endpoints under `/api/admin/tickets`:

- `GET /api/admin/tickets`: paginated list with user, status, type, and
  priority filters;
- `GET /api/admin/tickets/:id`: detail and full conversation;
- `POST /api/admin/tickets/:id/messages`: append an admin reply;
- `PATCH /api/admin/tickets/:id`: change status or priority;
- `GET /api/admin/tickets/:id/attachments/:attachment_id`: authorized
  download.

Request bodies contain only subject/category/priority/body and attachment
references. They never accept balance adjustments, refund amounts, API keys,
tokens, or arbitrary file paths.

## Conversation state machine

- Creation: `open`, unread for admin.
- Admin reply: `pending_user`, unread for user.
- User reply: `pending_admin`, unread for admin.
- Admin close: `closed`, both unread counters cleared.
- Admin reopen: `pending_admin` and visible to the user.

Appending a message and updating status, timestamps, and unread counters happen
in one database transaction. Repeated submissions must not duplicate a message
when the client retries with the same request id, using the project's existing
request-id/idempotency convention where available.

## Attachment security

Attachments are stored outside public static paths with a per-ticket private
key. The download handler verifies the authenticated user owns the ticket or
has admin permission before opening the file. It sanitizes the display name,
rejects path traversal, sniffs the content type, enforces a fixed maximum file
count and size, and removes incomplete temporary files. The initial UI may
match the reference limit of five files with a 50 MB maximum per file, subject
to the server's global request-body limit.

Ticket bodies and attachments must not be written to access logs. Error
responses contain no filesystem path or internal storage key.

## Frontend behavior

Use the current `web/default` routing, React Query API helpers, Base UI
components, existing brand theme, and i18n conventions.

- Add a user-visible `Ticket Center` item in the personal navigation.
- Add an admin-visible `Tickets` item in the admin navigation.
- The list uses the existing table and pagination patterns.
- The create dialog has the reference fields: category (general/refund),
  subject, priority, description, and attachments.
- The detail view renders a chronological conversation with role labels,
  timestamps, attachment links, unread state, and a reply composer.
- A closed ticket disables the user composer and explains that an admin must
  reopen it. Admins retain reply and reopen controls.
- All visible strings use `useTranslation()` and the project's locale files.

No page replaces the existing branded home, studio, canvas, pricing, or admin
pages. The ticket routes are additive.

## Validation and errors

- Reject empty subjects/bodies, unsupported categories or priorities, oversized
  attachments, and replies to tickets the caller cannot access.
- A user reply to a closed ticket returns a stable conflict error without
  exposing ticket ownership details.
- Admin list/detail responses may include the user's display name/email under
  existing admin privacy conventions, but never API credentials or full token
  values.

## Verification and rollout

Backend tests cover model validation, ownership, admin authorization, atomic
create/reply/status transitions, pagination, closed-ticket conflicts,
attachment path and MIME checks, and cross-database migration compatibility.

Frontend tests/build cover navigation visibility, create/detail/reply flows,
closed state, attachment errors, and all locale keys.

Run the complete Go test suite and frontend build in the isolated candidate.
Open the local branded UI for user review before any production build. The
production database receives no ticket migration until the candidate is
approved; the production rollout must keep a pre-migration backup and allow a
container rollback without restoring or changing billing data.
