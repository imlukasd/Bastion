# Bastion — Invariants

| Field | Value |
| --- | --- |
| Document | Correctness Contract |
| Status | Draft v4.1, for owner review |
| Last updated | 2026-10-26 |
| Related | [PRD](01-prd.md) · [Architecture](02-architecture.md) · [Technical Design](03-technical-design.md) |

This document defines **what must always be true** in Bastion. The system is done when all 17 invariants hold while both products are overloaded, have processes killed, receive wrong answers from the bank and the card network, lose the network between them, face disputes and resale races, and have an event cancelled mid on-sale (PRD §11).

Every invariant is protected twice: **prevent** at the point of write (preferably with a PostgreSQL constraint or trigger) and **detect** with an independent check that shares no code with the writer (the read-only auditor, property tests, chaos tests, k6 `teardown()`). **One violation fails the run.**

---

## 1. How to read an invariant

| Field | Meaning |
| --- | --- |
| **Scope** | Bastion Pay (`bastion_pay`), Bastion Tickets (`bastion_tickets`), or both |
| **Statement** | The property, in terms of stored data |
| **Why** | The real-world failure it prevents |
| **Prevent** | What makes a violating write impossible or abort |
| **Detect** | The auditor's queries (sketched in SQL) |
| **Verified by** | Property and model tests, concurrency tests (CC*), chaos scenarios (C*) |
| **Pitfalls** | The mistakes most likely to break it |

## 2. Definitions

### 2.1 Ledgers

Each company has its own ledger (Technical Design §3). Postings are signed (**+ debit, − credit**); a pending entry has two postings and resolves once.

```
posted(a)     = Σ amount    of postings of a in posted entries
pending_dr(a) = Σ amount    of postings of a with amount > 0 in unresolved pending entries
pending_cr(a) = Σ (−amount) of postings of a with amount < 0 in unresolved pending entries
available(a)  = −posted − pending_dr   (credit-normal)   |   posted − pending_cr   (debit-normal)
```

### 2.2 E-wallet liabilities (Bastion Pay)

`liabilities = Σ CUSTOMER_WALLET + Σ MERCHANT_INCOMING + Σ max(MERCHANT_BALANCE, 0) + UNCLAIMED_FUNDS` (posted). `MERCHANT_CARD_PENDING` is **not** e-wallet money: it is backed by `PSP_RECEIVABLE` until the card network settles it. The safeguarding surplus is `posted(SAFEGUARD_BANK) − liabilities`.

### 2.3 Inventory, tickets and ownership (Bastion Tickets)

- Zones: `capacity`, `reserved`, `sold`; seated zones also have seats (`AVAILABLE`, `HELD`, `SOLD`) and an immutable `reservation_seats` history. Inventory changes only through reserve, commit, release, expire and uncommit (after a refused capture).
- Tickets: one per purchased primary unit; status `VALID`, `LISTED`, `REFUND_PENDING`, `REVOKED` or `VOID`; **live** unless `VOID`; each has `owner_fan_id` and `owner_version`.
- Ownership changes only through `orders.ticket_transfers` rows: `SALE` (written with the resale's capture) and `UNWIND` (a lost dispute on a resale purchase).
- Listings: `ACTIVE`, `LOCKED` (one buyer, until `locked_until`), `SELLING` (the buyer's payment is authorized and the sale is committed; capture in progress), `SOLD`, `CANCELLED`, `EXPIRED`; **open** means `ACTIVE`, `LOCKED` or `SELLING`.
- Card attempts: every card confirmation of a Payment Intent is an `AUTHORIZE` operation numbered by the intent's `attempt_seq`; it is **live** unless `FAILED`.

### 2.4 Other terms

| Term | Meaning |
| --- | --- |
| **Customer-initiated debit** | P2P send, wallet payment hold, cash-out hold |
| **Driven object** | Owned by a job under a lease (orders in `AUTHORIZED`, `COMMITTED`, `CAPTURING`, `CANCELLING`; operations in flight) |
| **Quiescent** | No driven objects, no live Redis claims, no `LOCKED` or `SELLING` listings, no pending refunds, card operations or webhook deliveries, empty outbox backlogs, no transfers in flight between the companies |

## 3. Summary

| Code | Scope | Statement (short) | Primary prevention | Cadence |
| --- | --- | --- | --- | --- |
| **M1** | both | Every entry sums to 0 per currency | Deferred constraint triggers | 5 s |
| **M2** | both | Balances = Σ postings | Balances writable only by triggers | 30 s; full when quiescent |
| **M3** | both | Postings append-only; sealed history hash-chained | Privileges + trigger + sealer | 10 s |
| **M4** | both | Accounts within bounds; outflows never exceed availability; disbursements never use credit | `CHECK within_bound`; holds; guarded disbursements | 5 s |
| **M5** | both | No double charge | Partial unique indexes; one live card attempt, one capture, one void; guarded amounts | 30 s |
| **M6** | both | One idempotency key → one effect | `UNIQUE` at every layer | 30 s |
| **M7** | both | External records reconcile, or differences are explained | Detection only | Per file; daily |
| **W1** | Pay | Escrow covers all e-wallet balances | Credit after bank confirmation; card funds pending; surplus floor | Daily |
| **W2** | Pay | Limits never exceeded, holds included | `CHECK within_cap` | 5 s |
| **W3** | Pay | No customer debit on a non-`ACTIVE` wallet; bank money only via the holder's accounts | Wallet guard under a share lock | 30 s |
| **I1** | Tickets | No oversell; no double-booked seat | Guarded updates + `CHECK`s + one live ticket per seat | 5 s |
| **I2** | Tickets | Inventory conserved; counters equal facts; Redis converges | Five operations | 30 s; Redis when quiescent |
| **I3** | Tickets | Per-fan limit, holds and resale included | Quota row + `CHECK within_limit` | 5 s |
| **I4** | both | Legal transitions only; no commit or sale after expiry | FSM trigger + opposite time guards | 30 s |
| **I5** | Tickets | One owner per live ticket; ownership changes only through completed resales and single-hop unwinds; one open listing; price ≤ face | Guarded transfer + transfer rows + partial unique index + `CHECK price_cap` | 30 s |
| **X1** | Tickets | Primary money, seats and tickets agree | Capture trigger; tickets in the capture transaction; automatic uncommit after a refused capture | 30 s, grace 5 min |
| **X2** | Tickets | Resale money and ownership agree | Listing committed (`SELLING`) before capture; ownership and seller credit only in the capture transaction | 30 s |

---

## 4. Money invariants (each company's books)

### M1 — Every journal entry sums to zero

**Scope.** Both ledgers. **Statement.** For every entry: Σ postings = 0 per currency; ≥ 2 postings; exactly 2 if pending. Corollaries: Σ posted = 0 and Σ pending_dr = Σ pending_cr over each ledger.

**Why.** Money cannot appear or vanish inside the books.

**Prevent:** deferred triggers `entry_balanced`, `posting_balanced`; `Batch.Flush` validates in Go; every split is exact (MDR, commission and resale fee use `floor`, the remainder goes to the other party).

**Detect:**

```sql
SELECT e.id FROM ledger.entries e JOIN ledger.postings p ON p.entry_id = e.id
GROUP BY e.id, e.phase
HAVING SUM(p.amount) <> 0 OR COUNT(*) < 2 OR (e.phase = 'pending' AND COUNT(*) <> 2);

SELECT currency FROM ledger.accounts GROUP BY currency
HAVING SUM(posted) <> 0 OR SUM(pending_dr) <> SUM(pending_cr);
```

**Verified by:** property tests per ledger; CC1, CC11; C6, C12, C13.

**Pitfalls:** splitting with fractions of a total; posting two halves in different transactions.

### M2 — Every balance equals the sum of its postings

**Scope.** Both. **Statement.** `posted`, `pending_dr`, `pending_cr` equal §2.1 for every account.

**Why.** Every bound and the safeguarding surplus read cached balances.

**Prevent:** no application role may update balance columns; only `SECURITY DEFINER` triggers change them, with the postings; resolutions keyed by the pending entry.

**Detect:**

```sql
WITH open_pending AS (
  SELECT e.id FROM ledger.entries e
  LEFT JOIN ledger.pending_resolutions r ON r.pending_entry_id = e.id
  WHERE e.phase = 'pending' AND r.pending_entry_id IS NULL
), calc AS (
  SELECT p.account_id,
         COALESCE(SUM(p.amount)  FILTER (WHERE e.phase = 'posted'), 0)                AS posted,
         COALESCE(SUM(p.amount)  FILTER (WHERE o.id IS NOT NULL AND p.amount > 0), 0)  AS pending_dr,
         COALESCE(-SUM(p.amount) FILTER (WHERE o.id IS NOT NULL AND p.amount < 0), 0)  AS pending_cr
  FROM ledger.postings p JOIN ledger.entries e ON e.id = p.entry_id
  LEFT JOIN open_pending o ON o.id = e.id GROUP BY p.account_id
)
SELECT a.id FROM ledger.accounts a LEFT JOIN calc c ON c.account_id = a.id
WHERE (a.posted, a.pending_dr, a.pending_cr)
   <> (COALESCE(c.posted, 0), COALESCE(c.pending_dr, 0), COALESCE(c.pending_cr, 0));
```

**Verified by:** property tests; CC1, CC5, CC6, CC11; C6, C12, C13.

### M3 — Postings are append-only; sealed history is tamper-evident

**Scope.** Both. **Statement.** Entries, postings, resolutions and seals are never updated or deleted; entries older than 60 s are sealed; each chain verifies.

**Prevent:** privileges; `reject_mutation`; corrections as new entries (P31, T13).

**Detect:** incremental re-hashing; unsealed age (S3); privilege and trigger drift (`has_table_privilege` for `pay_app` and `tickets_app` must be false for `UPDATE`/`DELETE` on ledger tables).

**Verified by:** mutation-attempt integration tests; a tamper test that breaks the chain at exactly the edited `seq`.

**Pitfalls:** chaining inside the write transaction (AD-18); migrations that rewrite ledger rows.

### M4 — Bounds and availability

**Scope.** Both. **Statement:**

- Pay: `CUSTOMER_WALLET` available ≥ 0; `MERCHANT_BALANCE` available ≥ −refund credit limit.
- Tickets: `ORGANIZER_AVAILABLE` ≥ −overdraft limit; `SELLER_PAYABLE` ≥ 0.
- Every outflow (cash-out, settlement, organizer or seller payout, wallet hold, card refund) is held against availability when created.
- A disbursement never exceeds the merchant's available balance at the time it is made (no credit).

**Why.** E-wallets may not extend credit to customers; merchant and organizer debts are capped; seller proceeds cannot be paid twice; disbursements cannot pay out money the merchant never received.

**Prevent:** `CHECK within_bound`; holds first; accounts locked once per transaction in id order; bounded accounts never sharded; forced debits beyond the bound go to `CUSTOMER_DEBT`, `MERCHANT_DEBT` or `CREDIT_LOSSES`; disbursements check `available ≥ amount` under the account lock.

**Detect:**

```sql
SELECT id, code, owner_id FROM ledger.accounts
WHERE lower_bound IS NOT NULL
  AND (CASE normal_side WHEN 'credit' THEN -posted - pending_dr ELSE posted - pending_cr END) < lower_bound;

-- bastion_pay: no disbursement drove the merchant's available balance below zero
SELECT d.id FROM merchants.disbursements d
JOIN ledger.entries e ON e.kind = 'disbursement' AND e.reference_id = d.id
WHERE (SELECT -SUM(p.amount) FROM ledger.postings p JOIN ledger.entries x ON x.id = p.entry_id
       JOIN ledger.accounts a ON a.id = p.account_id
       WHERE a.code = 'MERCHANT_BALANCE' AND a.owner_id = d.merchant_id
         AND x.phase = 'posted' AND x.xact_id <= e.xact_id) < 0;          -- sketch: balance right after it
```

**Verified by:** CC5, CC6; C10, C13, C15, C18.

**Pitfalls:** counting incoming holds as available; sharding a bounded account; checking in Go without the lock.

### M5 — No double charge

**Scope.** Both. **Statement:**

- Pay, per intent: at most one non-cancelled intent per `(merchant, reference)`; at most one live card attempt at a time and at most one successful authorization; one `CAPTURE` and one `VOID` card operation, both for the authorized attempt; a new attempt only after the previous one definitively failed; `amount_captured ≤ amount`; `amount_refunded + amount_refund_pending + amount_disputed ≤ amount_captured`; `amount_refunded = Σ succeeded refunds`; a captured intent has exactly one capture entry (`pi_wallet_capture` or `pi_card_capture`).
- Pay, funding: one bank debit per cash-in, one charge per top-up, one bank transfer per cash-out and settlement, one disbursement per `(merchant, reference)`.
- Tickets: at most one non-cancelled payment per order; refunded + pending + disputed ≤ captured; a ticket is in at most one refund at a time and refunded at most once.

**Why.** Customers retry, merchants retry timed-out captures, and workers retry after crashes; none of it may move money twice. A card authorized twice holds a customer's money twice; a stale message from an old attempt must not reopen the intent while a newer attempt is in flight.

**Prevent:** `one_live_intent_per_reference`; `one_live_authorization` and `one_capture_one_void` on card operations; attempt numbers allocated by the guarded statement that claims the intent; outcomes recorded on the operation row first (`status IN ('PENDING','UNKNOWN')`), then on the intent with `attempt_seq = n`; unknown attempts resolved by lookup, never assumed failed; amount `CHECK`s; guarded updates; `one_live_payment_per_order`; tickets only from `VALID`; `UNIQUE (kind, reference_id)` on ledger entries; derived keys; unknown outcomes resolved by lookup.

**Detect:**

```sql
-- bastion_pay
SELECT merchant_id, merchant_reference FROM intents.payment_intents
WHERE status <> 'CANCELED' GROUP BY 1, 2 HAVING COUNT(*) > 1;

SELECT pi.id FROM intents.payment_intents pi
WHERE pi.amount_refunded <> (SELECT COALESCE(SUM(r.amount), 0) FROM intents.refunds r
                             WHERE r.payment_intent_id = pi.id AND r.status = 'SUCCEEDED');

SELECT pi.id FROM intents.payment_intents pi
LEFT JOIN ledger.entries e ON e.kind IN ('pi_wallet_capture','pi_card_capture') AND e.reference_id = pi.id
WHERE (pi.status = 'SUCCEEDED') <> (e.id IS NOT NULL);

SELECT payment_intent_id FROM intents.card_operations                 -- tripwire behind one_live_authorization
WHERE op = 'AUTHORIZE' AND status = 'SUCCEEDED' GROUP BY 1 HAVING COUNT(*) > 1;

SELECT pi.id FROM intents.payment_intents pi                           -- an intent waiting for confirmation with a live attempt
JOIN intents.card_operations o ON o.payment_intent_id = pi.id AND o.op = 'AUTHORIZE' AND o.status IN ('PENDING','UNKNOWN')
WHERE pi.status = 'REQUIRES_CONFIRMATION';

-- bastion_tickets
SELECT order_id FROM payments.payments WHERE status <> 'CANCELED' GROUP BY 1 HAVING COUNT(*) > 1;
```

External truth in tests: `pspsim` reports ≤ 1 successful authorization per intent across all its attempts, ≤ 1 capture per intent, ≤ 1 charge per top-up key, Σ refunds ≤ captured; `banksim` ≤ 1 effect per key.

**Verified by:** property tests; CC2; C1, C2, C5, C10, C12, C13, C14, C17, C20, C21.

**Pitfalls:** retrying an unknown authorization with a new key; starting a new attempt while one is unknown; guarding an outcome only on the intent's status (a stale decline then reopens it); letting a second checkout create a second intent; treating a webhook and a response as two captures.

### M6 — One idempotency key, one effect

**Scope.** Both. **Statement.** For every `(scope, key)` in any API, the effect happens once; replays return an equivalent response; mismatched requests are rejected.

**Prevent:**

| Layer | Mechanism |
| --- | --- |
| APIs | `PRIMARY KEY (scope, key)`, completed with the effect; request hash; lease |
| Pay objects | One live intent per reference; one live card attempt, one capture and one void per intent; `UNIQUE` partner ids; one disbursement per merchant reference |
| Tickets objects | `UNIQUE (idem_scope, idem_key)` on orders; one live payment per order; `UNIQUE (order_line_id, unit_no)` on tickets; `PRIMARY KEY (ticket_id, to_version)` on transfers; one open listing per ticket |
| Ledgers | `UNIQUE (kind, reference_id)` |
| Calls across boundaries | Derived keys |
| Webhooks and events | `outbox.inbox (consumer, event_id)` with the effect |
| Redis | Claim record per key |

**Detect:** tripwire queries on every unique key above; completed keys pointing at existing resources; per-key effect counts at partners in tests.

**Verified by:** CC2 (100 identical P2P transfers, wallet and card confirmations, captures, reservations, listing purchases, checkouts); C1, C2, C9, C12, C14, C17, C20.

**Pitfalls:** storing responses of transient failures; reading idempotency state from a replica; generating partner keys at call time.

### M7 — External records reconcile

**Scope.** Both. **Statement:**

| Instance | Product | Record | Identity |
| --- | --- | --- | --- |
| M7a | Pay | Card settlement batch | `net_transferred − expected_net = Σ amount_delta(batch exceptions)`; every captured card intent released exactly once, by the batch that settled it; a capture line for an intent Bastion cancelled is an exception (`CAPTURE_AFTER_CANCEL`), never a state change |
| M7b | Pay | Escrow and operating statements | `ledger movement − statement net movement = Σ amount_delta(statement exceptions)`; every line matched or excepted |
| M7c | Tickets | Pay settlement reports and Tickets' bank statement | Every report line matches a Tickets record; report arithmetic holds; bank credit = settled amount; differences explained |

**Prevent:** not possible (external data). Matchers force explanation: a batch, statement or report cannot be reconciled until its identity holds.

**Detect:**

```sql
-- M7a, M7b (bastion_pay); M7c uses the same shapes in bastion_tickets
SELECT b.id FROM safeguarding.psp_batches b
WHERE b.status = 'POSTED' AND (b.net_transferred - b.expected_net)
   <> COALESCE((SELECT SUM(x.amount_delta) FROM safeguarding.exceptions x WHERE x.source = 'PSP' AND x.batch_id = b.id), 0);

SELECT s.id FROM safeguarding.bank_statements s
WHERE s.status = 'RECONCILED' AND s.ledger_movement - (s.closing_balance - s.opening_balance)
   <> COALESCE((SELECT SUM(x.amount_delta) FROM safeguarding.exceptions x WHERE x.source = 'BANK' AND x.batch_id = s.id), 0);

SELECT l.statement_id, l.line_no FROM safeguarding.bank_statement_lines l
WHERE l.match_status <> 'MATCHED'
  AND NOT EXISTS (SELECT 1 FROM safeguarding.exceptions x WHERE x.batch_id = l.statement_id AND x.line_no = l.line_no);

SELECT id FROM intents.payment_intents                                -- released at most once, only after settlement
WHERE card_funds_released_at IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM safeguarding.psp_lines l
                  WHERE l.type = 'CAPTURE' AND l.matched_id = payment_intents.id AND l.match_status = 'MATCHED');
```

**Verified by:** C8 (dropped, duplicated, skewed lines in each record); unit tests against hand-computed files.

**Pitfalls:** matching by amount; reconciling by calendar day; forgetting returns, chargebacks, dispute fees, card releases.

---

## 5. Wallet invariants (Bastion Pay)

### W1 — Customer funds are safeguarded

**Statement.** For every business day D with an imported escrow statement: `closing_balance(D) ≥ e-wallet liabilities from postings committed before the cut-off` (§2.2); a statement has been imported and checked for every business day older than one day.

**Why.** E-wallet providers must keep escrow balances not lower than issued wallet balances (PRD §2.2).

**Prevent:**

- Cash-ins credit wallets only after the bank confirms the debit (AD-32).
- Card captures credit `MERCHANT_CARD_PENDING`, not e-wallet money, until the card settlement brings the cash into escrow (AD-33).
- Card top-ups and revenue sweeps are refused when they would exceed the conservative surplus floor (AD-28).
- Money leaves escrow only by debiting a liability in the same transaction (cash-outs, settlements).
- Forced floor reductions (a card settlement netting more disputes and fees than it releases) page immediately and halt top-ups, sweeps and settlements until capital is injected.

**Detect:**

```sql
WITH s AS (SELECT id, statement_date, closing_balance, cutoff_at
           FROM safeguarding.bank_statements WHERE account_role = 'ESCROW'),
bal AS (
  SELECT s.id AS statement_id, p.account_id, -SUM(p.amount) AS balance
  FROM s JOIN ledger.entries e ON e.phase = 'posted' AND e.created_at < s.cutoff_at
  JOIN ledger.postings p ON p.entry_id = e.id JOIN ledger.accounts a ON a.id = p.account_id
  WHERE a.code IN ('CUSTOMER_WALLET','MERCHANT_INCOMING','MERCHANT_BALANCE','UNCLAIMED_FUNDS')
  GROUP BY s.id, p.account_id
)
SELECT s.statement_date FROM s JOIN bal b ON b.statement_id = s.id
GROUP BY s.id, s.statement_date, s.closing_balance
HAVING s.closing_balance < SUM(GREATEST(b.balance, 0));

SELECT d::date FROM generate_series(current_date - 30, current_date - 2, '1 day') d     -- liveness
WHERE NOT EXISTS (SELECT 1 FROM safeguarding.bank_statements s
                  WHERE s.account_role = 'ESCROW' AND s.statement_date = d::date AND s.w1_checked_at IS NOT NULL);
```

Entry `created_at` is the transaction start, so in-flight transactions at the cut-off may fall on either side; the effect is bounded by seconds of activity and absorbed by the capital buffer. S15 checks the floor stays conservative.

**Verified by:** C15, C16, the `safeguarding_stress` load test, and a model property: `posted(SAFEGUARD_BANK) − liabilities ≥ floor` after any operation sequence.

**Pitfalls:** crediting a cash-in early; treating card captures as available; sweeping without decrementing the floor.

### W2 — Limits are never exceeded

**Statement.** For every `(customer, period, period_start)`: `used + reserved ≤ cap`; `reserved = Σ amount` of the customer's wallet intents in `REQUIRES_CAPTURE` stored on this period; `used = Σ P2P sent + Σ captured wallet intents`, attributed to their stored periods. Card payments never count.

**Prevent:** `CHECK within_cap`; both rows locked `DAY` before `MONTH` and updated with a guard in the hold or transfer transaction; periods computed in the database in `Asia/Ho_Chi_Minh`; intents remember their rows.

**Detect:**

```sql
SELECT customer_id, period, period_start FROM limits.usage WHERE used + reserved > cap;

SELECT u.customer_id, u.period, u.period_start FROM limits.usage u
LEFT JOIN LATERAL (
  SELECT COALESCE(SUM(pi.amount) FILTER (WHERE pi.status = 'REQUIRES_CAPTURE'), 0)   AS reserved,
         COALESCE(SUM(pi.amount_captured) FILTER (WHERE pi.status = 'SUCCEEDED'), 0) AS captured
  FROM intents.payment_intents pi
  WHERE pi.method = 'wallet' AND pi.payer_customer_id = u.customer_id
    AND (CASE u.period WHEN 'DAY' THEN pi.limit_day ELSE pi.limit_month END) = u.period_start) i ON true
LEFT JOIN LATERAL (
  SELECT COALESCE(SUM(t.amount), 0) AS sent FROM transfers.p2p t
  JOIN customers.wallets w ON w.id = t.from_wallet
  WHERE w.customer_id = u.customer_id
    AND (CASE u.period WHEN 'DAY' THEN t.limit_day ELSE t.limit_month END) = u.period_start) p ON true
WHERE u.reserved <> i.reserved OR u.used <> i.captured + p.sent;
```

**Verified by:** CC8; property tests.

**Pitfalls:** `SELECT SUM` in Go; UTC periods; converting a reservation on the capture day; locking the two rows in different orders; counting card payments.

### W3 — Wallet status and bank destinations are respected

**Statement:** no customer-initiated debit commits after the wallet's latest transition away from `ACTIVE` (unless it returned to `ACTIVE` first); every cash-in and cash-out uses a linked account owned by the holder and `VERIFIED` when the operation was created; a wallet becomes `ACTIVE` only after a verified link.

**Prevent:** customer debits take the wallet row `FOR SHARE` with `status = 'ACTIVE'`; a freeze needs the row exclusively; linked-account checks in the creating transaction; activation in the same transaction as the first verified link.

**Detect** (commit order from `track_commit_timestamp`):

```sql
WITH debits AS (
  SELECT e.id, pg_xact_commit_timestamp(xid(e.xact_id)) AS committed_at, w.id AS wallet_id
  FROM ledger.entries e
  JOIN ledger.postings p ON p.entry_id = e.id AND p.amount > 0
  JOIN ledger.accounts a ON a.id = p.account_id AND a.code = 'CUSTOMER_WALLET'
  JOIN customers.wallets w ON w.customer_id = a.owner_id
  WHERE e.kind IN ('p2p','pi_wallet_hold','cashout_hold')
), changes AS (
  SELECT object_id AS wallet_id, to_status, pg_xact_commit_timestamp(xid(xact_id)) AS committed_at
  FROM fsm.state_events WHERE object_type = 'wallet'
)
SELECT d.id FROM debits d
WHERE (SELECT c.to_status FROM changes c WHERE c.wallet_id = d.wallet_id AND c.committed_at < d.committed_at
       ORDER BY c.committed_at DESC LIMIT 1) IS DISTINCT FROM 'ACTIVE';

SELECT co.id FROM funding.cashouts co JOIN customers.linked_accounts la ON la.id = co.linked_account_id
WHERE la.customer_id <> co.customer_id OR la.verified_at IS NULL OR la.verified_at > co.created_at;
```

**Verified by:** CC9, CC10; C15.

**Pitfalls:** checking status without a lock; skipping the P2P receiver's status; caching status in Redis.

---

## 6. Inventory and ownership invariants (Bastion Tickets)

### I1 — No oversell and no double-booked seat

**Statement.** For every zone, `0 ≤ reserved`, `0 ≤ sold`, `reserved + sold ≤ capacity`; every seat is taken by at most one live reservation (`HELD`, or `COMMITTED` and not uncommitted); every seat has at most one live ticket.

**Prevent:** guarded zone `UPDATE` with EvalPlanQual re-check + `CHECK no_oversell`; seats changed from `AVAILABLE` only under a guard after locking (`(zone_id, id)` order for seat pick, `SKIP LOCKED` for best available); `CHECK holder_matches_status`; partial unique index `one_live_ticket_per_seat`. Resale never creates tickets, so it cannot create a second ticket for a seat.

**Detect:**

```sql
SELECT event_id, zone_id FROM inventory.zone_inventory
WHERE reserved < 0 OR sold < 0 OR reserved + sold > capacity;

SELECT rs.seat_id FROM inventory.reservation_seats rs
JOIN inventory.reservations r ON r.id = rs.reservation_id
LEFT JOIN inventory.uncommits u ON u.reservation_id = r.id
WHERE r.status = 'HELD' OR (r.status = 'COMMITTED' AND u.reservation_id IS NULL)
GROUP BY rs.seat_id HAVING COUNT(*) > 1;

SELECT seat_id FROM orders.tickets WHERE seat_id IS NOT NULL AND status <> 'VOID'
GROUP BY seat_id HAVING COUNT(*) > 1;
```

**Verified by:** CC3, CC7; C3, C7, C11.

**Pitfalls:** read-then-write; locking picked seats in request order; trusting Redis or a stale seat map.

### I2 — Inventory is conserved; counters equal facts; Redis converges

**Statement:** zone `reserved = Σ HELD reservations`, `sold = Σ commits − Σ uncommits`; for seated zones `reserved = #HELD seats`, `sold = #SOLD seats`, every taken seat agrees with its reservation and history row, and a seated reservation took exactly `qty` seats; when quiescent, Redis counters equal PostgreSQL and claim sets are empty.

**Prevent:** only the five operations of Technical Design §23.1 change counters and seats, each with its reservation transition; reconciler and idempotent give-backs for Redis.

**Detect:**

```sql
WITH held AS (SELECT event_id, zone_id, SUM(qty) q FROM inventory.reservations WHERE status = 'HELD' GROUP BY 1, 2),
sold AS (SELECT r.event_id, r.zone_id, SUM(c.qty) - COALESCE(SUM(u.qty), 0) q
         FROM inventory.commits c JOIN inventory.reservations r ON r.id = c.reservation_id
         LEFT JOIN inventory.uncommits u ON u.reservation_id = c.reservation_id GROUP BY 1, 2)
SELECT i.event_id, i.zone_id FROM inventory.zone_inventory i
LEFT JOIN held h USING (event_id, zone_id) LEFT JOIN sold s USING (event_id, zone_id)
WHERE i.reserved <> COALESCE(h.q, 0) OR i.sold <> COALESCE(s.q, 0);

SELECT s.id FROM inventory.seats s LEFT JOIN inventory.reservations r ON r.id = s.reservation_id
WHERE (s.status = 'HELD' AND r.status IS DISTINCT FROM 'HELD')
   OR (s.status = 'SOLD' AND r.status IS DISTINCT FROM 'COMMITTED')
   OR (s.status <> 'AVAILABLE' AND NOT EXISTS (SELECT 1 FROM inventory.reservation_seats rs
                                              WHERE rs.reservation_id = s.reservation_id AND rs.seat_id = s.id));
-- plus: seated zone counters = seat counts; seated reservations took exactly qty seats
```

**Verified by:** C3, C7, C13; CC3, CC7.

### I3 — Per-fan purchase limit

**Statement.** For every `(fan, event)`: `reserved_qty + purchased_qty ≤ limit_qty`; `reserved_qty = Σ HELD primary reservations + #resale lines the fan is still buying` (line `RESERVED` or `COMMITTED`, order not `COMPLETED`); `purchased_qty = #live tickets the fan owns for the event + units of the fan's primary orders committed but not yet issued`.

**Why.** Limits blunt scalpers; counting holds, locks and resale purchases closes the "many tabs" and "buy back on resale" loopholes.

**Prevent:** `CHECK within_limit`; guarded updates in the same transaction as the fact they count: Tx B and the listing lock (`reserved + q`); Tx E for primary orders (`reserved − q`, `purchased + q`); Tx G for resales (buyer `reserved − 1`, `purchased + 1`; seller `purchased − 1`); releases, expiries, the lock sweeper and Tx X (`reserved − q`, or `purchased − q` after an uncommit); ticket voiding on a cancelled event (current owner `purchased − 1`); unwinds (buyer `− 1`, seller `+ 1`).

**Detect:**

```sql
SELECT q.fan_id, q.event_id FROM inventory.fan_quota q
LEFT JOIN LATERAL (
  SELECT (SELECT COALESCE(SUM(r.qty), 0) FROM inventory.reservations r
           WHERE r.fan_id = q.fan_id AND r.event_id = q.event_id AND r.status = 'HELD')
       + (SELECT COUNT(*) FROM orders.order_lines l JOIN orders.orders o ON o.id = l.order_id
           WHERE o.kind = 'RESALE' AND o.fan_id = q.fan_id AND o.event_id = q.event_id
             AND l.status IN ('RESERVED','COMMITTED') AND o.status <> 'COMPLETED') AS held,
         (SELECT COUNT(*) FROM orders.tickets t
           WHERE t.owner_fan_id = q.fan_id AND t.event_id = q.event_id AND t.status <> 'VOID')
       + (SELECT COALESCE(SUM(l.qty), 0) FROM orders.order_lines l JOIN orders.orders o ON o.id = l.order_id
           WHERE o.kind = 'PRIMARY' AND o.fan_id = q.fan_id AND o.event_id = q.event_id
             AND l.status = 'COMMITTED' AND o.status IN ('COMMITTED','CAPTURING')) AS owned   -- committed, tickets not yet issued
) x ON true
WHERE q.reserved_qty + q.purchased_qty > q.limit_qty OR q.reserved_qty <> x.held OR q.purchased_qty <> x.owned;
```

**Verified by:** CC4, CC12; model test.

**Pitfalls:** counting only primary purchases; forgetting to decrement the seller on transfer; decrementing the original buyer instead of the current owner when a ticket is voided; counting per zone.

### I4 — State machines are respected

**Scope.** Both. **Statement.** Every status change of every FSM-guarded object in either product is a legal edge; creations start in initial states; nothing leaves a terminal state; **an expired reservation is never committed, an expired or cancelled listing lock never starts a sale (`LOCKED → SELLING`), and an elapsed capture window never captures a wallet hold**.

**Prevent:** `fsm.guard` trigger; guarded `UPDATE`s; opposite time guards on the same row (commit vs. reservation expiry, sale commit vs. lock expiry, capture vs. capture window, confirmation vs. confirmation deadline); stale or duplicate partner messages matched against the operation row first, so they cannot move an object backwards.

**Detect:**

```sql
SELECT e.id FROM fsm.state_events e
LEFT JOIN fsm.edges g ON (g.object_type, g.from_status, g.to_status) = (e.object_type, e.from_status, e.to_status)
WHERE e.from_status IS NOT NULL AND g.object_type IS NULL;

SELECT r.id FROM inventory.reservations r JOIN inventory.commits c ON c.reservation_id = r.id
WHERE c.created_at >= r.expires_at;                                        -- tickets

SELECT l.id FROM resale.listings l                                          -- tickets: the sale's commit point
JOIN fsm.state_events e ON e.object_type = 'listing' AND e.object_id = l.id AND e.to_status = 'SELLING'
WHERE e.at >= l.locked_until;                                               -- locked_until is frozen from SELLING on

SELECT pi.id FROM intents.payment_intents pi                               -- pay, wallet intents
JOIN fsm.state_events e ON e.object_type = 'payment_intent' AND e.object_id = pi.id AND e.to_status = 'SUCCEEDED'
WHERE pi.method = 'wallet' AND e.at >= pi.capture_before;
```

Card captures found succeeded at the network after the window are recorded anyway (Technical Design §14.9): the network's fact wins, and the window applies only to starting a capture. A capture reported after the intent is `CANCELED` does not change any state; it is an M7a exception.

**Verified by:** model tests with stale webhooks and last-second operations; C2, C4, C6, C12, C13, C14, C19, C20, C21.

### I5 — Ticket ownership

**Statement:**

- Every live ticket has exactly one owner, and its `owner_version` equals 1 + the number of its transfer rows.
- Every transfer row is backed by a resale order: a `SALE` by a `COMPLETED` resale order of the same ticket, from the listing's seller to the order's buyer; an `UNWIND` only by a lost dispute on that order, back to the seller, on an event that is not cancelled, and only as the very next transfer after that order's `SALE` (a single hop).
- A ticket has at most one open listing; an open listing's ticket is `LISTED` and owned by the listing's seller at the listing's `seller_owner_version`; a `LISTED` ticket has an open listing.
- Every listing price is ≤ the ticket's face value.

**Why.** A resold ticket must work for exactly one person, the seller's old copy must stop working, and resale must not become scalping. Unwinds across several hops (A → B → C) would race with each other; refunds therefore never unwind (Technical Design §25.5).

**Prevent:** the guarded transfer `UPDATE … WHERE status = 'LISTED' AND owner_fan_id = $seller AND owner_version = $v` in the capture-recording transaction (Tx G), together with the transfer row and the listing's `SELLING → SOLD`; the guarded unwind `UPDATE … WHERE status = 'REVOKED' AND owner_fan_id = $buyer AND owner_version = $sale_version`; nothing but the buying order touches a `SELLING` listing; `PRIMARY KEY (ticket_id, to_version)`; `one_open_listing_per_ticket`; `CHECK price_cap`; the QR payload includes `owner_version` (Technical Design §22.5).

**Detect:**

```sql
SELECT t.id FROM orders.tickets t
WHERE t.owner_version <> 1 + (SELECT COUNT(*) FROM orders.ticket_transfers x WHERE x.ticket_id = t.id);

SELECT x.ticket_id, x.to_version FROM orders.ticket_transfers x
JOIN orders.orders o ON o.id = x.resale_order_id
JOIN orders.order_lines l ON l.order_id = o.id AND l.ticket_id = x.ticket_id
JOIN resale.listings li ON li.id = l.listing_id
WHERE o.kind <> 'RESALE'
   OR (x.kind = 'SALE'   AND (x.from_fan_id <> li.seller_fan_id OR x.to_fan_id <> o.fan_id))
   OR (x.kind = 'UNWIND' AND (x.from_fan_id <> o.fan_id OR x.to_fan_id <> li.seller_fan_id
                              OR NOT EXISTS (SELECT 1 FROM orders.ticket_transfers s          -- single hop
                                             WHERE s.ticket_id = x.ticket_id AND s.resale_order_id = o.id
                                               AND s.kind = 'SALE' AND s.to_version = x.to_version - 1)));

SELECT l.id FROM resale.listings l JOIN orders.tickets t ON t.id = l.ticket_id
WHERE l.status IN ('ACTIVE','LOCKED','SELLING')
  AND (t.status <> 'LISTED' OR t.owner_fan_id <> l.seller_fan_id OR t.owner_version <> l.seller_owner_version);

SELECT t.id FROM orders.tickets t WHERE t.status = 'LISTED'
  AND NOT EXISTS (SELECT 1 FROM resale.listings l WHERE l.ticket_id = t.id AND l.status IN ('ACTIVE','LOCKED','SELLING'));

SELECT id FROM resale.listings WHERE price > face_value;                   -- tripwire
```

**Verified by:** CC12 (500 buyers on one listing, seller withdrawing concurrently); property tests over random list/withdraw/buy/dispute sequences, including chains A → B → C; C13, C18, C19, C22.

**Pitfalls:** transferring by owner only (old QR copies keep working); voiding and re-issuing tickets (two tickets per seat over time); forgetting the version guard; letting a seller withdraw a locked listing; transferring before the capture is recorded (the buyer could relist an unpaid ticket); unwinding on refunds.

---

## 7. Money ↔ seats ↔ tickets ↔ ownership (Bastion Tickets)

### X1 — Primary money, seats and tickets agree

**Statement:**

- **(a)** Every succeeded capture of a primary order equals `Σ qty × (unit_price + unit_service_fee)` over its `COMMITTED` lines.
- **(b)** No seats stay committed behind a dead payment: every `COMMITTED` primary line belongs to an order that is `COMPLETED`, or whose intent at Bastion Pay is still live; once Pay has cancelled the intent, Tx X releases the seats within 5 minutes. (A capture the network is still retrying may keep seats committed for longer; that is liveness, watched by S9.)
- **(c)** For every line of a `COMPLETED` primary order of an event that is not cancelled: exactly `qty` tickets were issued, and `#VALID + #LISTED = qty − refunded_qty − refund_pending_qty − disputed_qty` (units the payer resold and later disputed stay valid with their new owners and are counted in `disputed_resold_qty`, not here). Lines of other primary orders have no tickets. Cancelled events are covered by S13.

**Why.** A valid ticket without money is a free seat; money without a valid ticket is a fan who paid and cannot get in; seats held behind a payment that will never come are lost sales.

**Prevent:** capture amount computed from committed lines in Tx F; the capture trigger; tickets inserted in the capture-recording transaction with `UNIQUE (order_line_id, unit_no)`; ticket status changes in the same transactions as the money (refunds, disputes); the intent deadline 30 s before the countdown ends; Tickets acts only on terminal intent events, and Tx X uncommits in the same transaction that records the cancellation.

**Detect:**

```sql
SELECT c.id FROM payments.captures c JOIN payments.payments p ON p.id = c.payment_id
JOIN orders.orders o ON o.id = p.order_id AND o.kind = 'PRIMARY'
WHERE c.status = 'SUCCEEDED'
  AND c.amount <> (SELECT COALESCE(SUM(l.qty * (l.unit_price + l.unit_service_fee)), 0)
                   FROM orders.order_lines l WHERE l.order_id = o.id AND l.status = 'COMMITTED');

-- (b) across both databases (the auditor reads both)
-- bastion_tickets: intents behind committed, unpaid seats
SELECT p.pay_intent_id FROM orders.order_lines l
JOIN orders.orders o ON o.id = l.order_id AND o.kind = 'PRIMARY' AND o.status IN ('COMMITTED','CAPTURING')
JOIN payments.payments p ON p.order_id = o.id AND p.status <> 'CANCELED'
WHERE l.status = 'COMMITTED';
-- bastion_pay: a violation if any of them was cancelled more than 5 minutes ago
SELECT pi.id FROM intents.payment_intents pi
JOIN fsm.state_events e ON e.object_type = 'payment_intent' AND e.object_id = pi.id AND e.to_status = 'CANCELED'
WHERE pi.id = ANY($intents) AND e.at < now() - interval '5 minutes';

SELECT l.id FROM orders.order_lines l JOIN orders.orders o ON o.id = l.order_id AND o.kind = 'PRIMARY'
JOIN catalog.events ev ON ev.id = o.event_id
LEFT JOIN LATERAL (SELECT COUNT(*) AS issued,
                          COUNT(*) FILTER (WHERE t.status IN ('VALID','LISTED')) AS usable
                   FROM orders.tickets t WHERE t.order_line_id = l.id) t ON true
WHERE (o.status = 'COMPLETED' AND t.issued <> l.qty)
   OR (o.status = 'COMPLETED' AND ev.status <> 'CANCELLED'
       AND t.usable <> l.qty - l.refunded_qty - l.refund_pending_qty - l.disputed_qty)
   OR (o.status <> 'COMPLETED' AND t.issued > 0);
```

**Verified by:** model test; CC2; C1, C4, C6, C12, C13, C14; C21 (a final refusal releases the seats automatically; a refusal the network retries and later captures issues the tickets exactly once and never releases the seats).

### X2 — Resale money and ownership agree

**Statement:**

- **(a)** A resale order is `COMPLETED` if and only if exactly one `SALE` transfer exists for it; its capture equals `unit_price + unit_service_fee` of its line. Both are written by the same transaction (Tx G), so the equivalence holds at every instant.
- **(b)** A `SALE` transfer belongs to an order whose payment is `SUCCEEDED` (no ownership without money), again in the same transaction.
- **(c)** The seller is credited (`SELLER_PAYABLE`, T2) only in that transaction. A resale order has an `UNWIND` transfer if and only if it lost a dispute while its buyer still owned the ticket and the event is not cancelled; the seller credit was then recovered by T6. An `EVENT_CANCELLED` refund reverses the seller credit without any transfer.
- **(d)** Every resale line's `party_net_held` + amounts already paid out to the seller + amounts reversed = `unit_price − unit_fee`.

**Why.** A buyer must not pay without receiving the ticket, receive it without paying, or have it taken back without a refund; a seller must not be paid for a sale that did not happen or was unwound.

**Prevent:** Tx E only commits the listing (`LOCKED → SELLING`, requiring `locked_until > now()`); ownership, the transfer row, the payment's `SUCCEEDED` and T2 are all written by the capture-recording transaction; the capture trigger checks the amount; a refused capture cancels the listing and leaves the ticket with the seller (Tx X); unwinds happen only in the transaction that records a lost dispute.

**Detect:**

```sql
SELECT o.id FROM orders.orders o
LEFT JOIN orders.ticket_transfers x ON x.resale_order_id = o.id AND x.kind = 'SALE'
WHERE o.kind = 'RESALE' AND (o.status = 'COMPLETED') <> (x.ticket_id IS NOT NULL);

SELECT x.ticket_id FROM orders.ticket_transfers x
LEFT JOIN payments.payments p ON p.order_id = x.resale_order_id AND p.status = 'SUCCEEDED'
WHERE x.kind = 'SALE' AND p.id IS NULL;

SELECT o.id FROM orders.orders o JOIN orders.order_lines l ON l.order_id = o.id
JOIN catalog.events ev ON ev.id = o.event_id
LEFT JOIN payments.payments p ON p.order_id = o.id AND p.status = 'SUCCEEDED'
LEFT JOIN payments.disputes d ON d.payment_id = p.id AND d.status = 'LOST'
LEFT JOIN orders.ticket_transfers u ON u.resale_order_id = o.id AND u.kind = 'UNWIND'
WHERE o.kind = 'RESALE'
  AND ((d.id IS NOT NULL AND l.disputed_qty = 1 AND ev.status <> 'CANCELLED') <> (u.ticket_id IS NOT NULL));
```

**Verified by:** model test; CC12; C13, C18, C19, C21, C22.

**Pitfalls:** moving ownership before the capture is recorded (the buyer could use or relist an unpaid ticket, and a refused capture would need an unwind); crediting the seller at lock time; unwinding on refunds, which races across resale chains.

---

## 8. Secondary checks

| Code | Scope | Check | Threshold |
| --- | --- | --- | --- |
| S1 | Pay | Oldest unswept wallet capture in `MERCHANT_INCOMING` ≤ 5 s | Page at 30 s |
| S2 | Tickets | `available(ORGANIZER_HELD(o)) ≥ 0` | Always |
| S3 | both | Oldest unsealed entry ≤ 60 s | Warn 10 s, page 60 s |
| S4 | both | Oldest unpublished outbox event; Pay: oldest pending webhook delivery, `FAILED` deliveries | Warn 60 s, page 300 s; any `FAILED` |
| S5 | Tickets | Redis drift not persisting during an on-sale | Page if > 0 for 30 s |
| S6 | Pay | `posted(PSP_RECEIVABLE)` = unsettled charges + unsettled card captures − unsettled refunds − unsettled chargebacks + unsettled reversals | Daily |
| S7 | both | No driven object with an expired lease for more than 2 min | Page |
| S8 | both | No pending ledger entry past its expiry by more than 5 min | Page |
| S9 | both | No unknown outcome (`*_UNKNOWN`, card operations `UNKNOWN`) older than 15 min; no capture the network is still retrying older than 24 h | Page |
| S10 | both | Ledger and FSM triggers present and enabled; privileges unchanged | Always |
| S11 | Tickets | Revenue and cost accounts equal their per-line and per-payment formulas (commission, service fees, resale fees, MDR, dispute fees) | Daily |
| S12 | Tickets | Per primary line of an event that is not cancelled: #`REFUND_PENDING` = `refund_pending_qty`; #`REVOKED` = `disputed_qty` of open disputes | 30 s |
| S13 | Tickets | Cancelled events, 24 h after cancellation: no live ticket except `REVOKED` ones under an open dispute; no open listing; every line of every `COMPLETED` order has `refunded_qty + disputed_qty + disputed_resold_qty = qty`, except lines with 3 failed refunds that have an open exception | Hourly |
| S14 | Tickets | No `HELD` reservation or `LOCKED` listing more than 30 s past expiry; no resale order holding a line whose listing is no longer locked by it for more than 30 s | Warn 5 s, page 30 s |
| S15 | Pay | `surplus_floor ≤ posted(SAFEGUARD_BANK) − liabilities` and `surplus_floor ≥ 0` | 30 s; page |
| S16 | cross | When quiescent: Tickets' `posted(PAY_RECEIVABLE)` = Pay's posted `MERCHANT_INCOMING + MERCHANT_CARD_PENDING + MERCHANT_BALANCE − MERCHANT_DEBT` for Tickets + settlements in transit − fundings in transit | After every load and chaos test; daily |
| S17 | Tickets | Per organizer and per seller: Σ `party_net_held` of `HELD` lines = posted held balance (`ORGANIZER_HELD`, `SELLER_PAYABLE`) | 30 s |
| S18 | Pay | Per merchant: `posted(MERCHANT_CARD_PENDING)` = Σ (`amount_captured − fee_amount`) of its captured card intents not yet released | 30 s |

## 9. Delivery guarantees

| Code | Guarantee | Mechanism |
| --- | --- | --- |
| E1 | Outbox events published at least once | Relay marks after `XADD` |
| E2 | Consumers apply each event once | Inbox with the effect; `XACK` after commit |
| E3 | Merchant webhooks delivered at least once within 72 h, or `FAILED` | Dispatcher with backoff |
| E4 | Incoming webhooks applied once | Signature + timestamp + inbox with the effect |

No invariant depends on a delivery: every correctness-relevant effect happens in the producing transaction, and polling and lookups by key back up every webhook between the companies.

---

## 10. Verification catalogue

### 10.1 Concurrency tests (`go test -race`, real PostgreSQL and Redis)

| Code | Product | Scenario | Must hold |
| --- | --- | --- | --- |
| CC1 | Pay | 1,000 concurrent P2P transfers in both directions | No deadlock; balances exact; M1, M2 |
| CC2 | both | 100 identical concurrent requests each: P2P, wallet and card confirmation, capture, reservation, listing purchase, checkout | One effect each; M5, M6 |
| CC3 | Tickets | 5,000 fans for 1,000 general-admission tickets | Exactly 1,000 sold; I1, I2 |
| CC4 | Tickets | One fan, 10 parallel single-ticket reservations, limit 4 | Exactly 4; I3 |
| CC5 | Pay | 50 concurrent debits exceeding one wallet's balance | Never negative; M4 |
| CC6 | Tickets | Organizer and seller payouts racing refunds and disputes | Bounds hold; M4, S17 |
| CC7 | Tickets | 1,000 fans pick the same 4 seats; 2,000 fans ask for 2 best-available seats in a 1,000-seat zone; mixed | One winner per seat; no deadlock; I1, I2 |
| CC8 | Pay | 20 concurrent transfers and wallet confirmations at the monthly cap, at 23:59:59 and month-end | Caps never exceeded; W2 |
| CC9 | Pay | 50 concurrent wrong-PIN attempts | ≤ 5 verifications |
| CC10 | Pay | A freeze racing 100 debits | No debit after the freeze; W3 |
| CC11 | Pay | 5,000 concurrent captures (wallet and card) for one merchant | Balances exact after sweep and release; M2, S1, S18 |
| CC12 | Tickets | 500 buyers lock the same listing while the seller withdraws it; 1,000 listings bought concurrently | One outcome per listing; one owner per ticket; I3, I5, X2 |

### 10.2 Chaos scenarios

Each is one automated test (Technical Design §34.2) ending with `/report?quiesce=60s` reporting **zero** violations, plus:

| Code | Scenario | Scenario-specific assertions |
| --- | --- | --- |
| C1 | Bank times out after debiting a cash-in | One debit; wallet credited once |
| C2 | Webhooks duplicated 3× and reordered: PSP → Pay, bank → Pay, Pay → Tickets | No regression; one effect per event |
| C3 | `kill -9` of `tickets-api` between Redis `admit` and Tx B | No PostgreSQL effect; Redis restored |
| C4 | Authorization webhook arrives after the countdown ended | Reservation expired; intent cancelled (hold released or authorization voided); no tickets |
| C5 | Concurrent refunds: same tickets; different tickets exceeding captured | One succeeds per ticket; Σ ≤ captured at both companies |
| C6 | PostgreSQL restarts (both databases) during an on-sale burst | Everything terminal ≤ 60 s after restart |
| C7 | Redis `FLUSHALL` mid on-sale | No oversell; rebuilt ≤ 5 s |
| C8 | Corrupted PSP file, escrow statement and Pay settlement report | Every difference explained; M7 identities hold |
| C9 | `relay` or `webhook-dispatcher` killed after sending, before marking | Duplicates deduplicated |
| C10 | Bank transfers (cash-out, settlement, organizer payout) time out after acceptance; a paid transfer is returned | One transfer per key; funds re-credited |
| C11 | Pay brownout (60% errors, +500 ms) mid on-sale | Breaker; reservations kept; no `5xx` storm |
| C12 | `kill -9` at every saga step in both products | Terminal ≤ 60 s; ≤ 1 effect per key at every partner |
| C13 | Event cancelled after resale chains (A → B → C) and after Tickets' Pay balance was settled; wallet and card refunds complete in random order; one card refund fails once | Pre-funding; every ticket `VOID`; every payment refunded exactly once regardless of completion order; every fan's net is 0; no `UNWIND` transfer |
| C14 | Partition between Tickets and Pay during capture | One capture; tickets issued once |
| C15 | Chargeback on a top-up after spending | Wallet at 0; debt; frozen; W1 holds |
| C16 | Card settlement delayed 3 days while top-ups and card payments continue | Top-ups refused at floor 0; card funds stay pending; W1 holds; release on settlement |
| C17 | Card authorization times out after the network authorized | One authorization at the network for `auth:{pi}:{n}`; no second attempt starts while it is unknown; intent reaches `REQUIRES_CAPTURE` or is voided |
| C18 | Disputes: on a primary card payment whose ticket was resold; on a resale card payment; one won, one lost | Resold ticket stays with the buyer and the payer's proceeds are recovered; resale dispute lost while the buyer still owns the ticket → ownership back to the seller in one hop; merchant debt at Pay when its balance is short; M4, I5, X2 |
| C19 | Listing lock expires exactly as the buyer's authorization arrives | Exactly one: `SELLING` + capture + transfer, or no sale + void |
| C20 | Card A declined, card B confirmed on the same intent; duplicate and reordered `authorization.*` webhooks for attempt 1 delivered while attempt 2 is in flight | Attempt 2's outcome recorded; the intent never returns to `REQUIRES_CONFIRMATION` while attempt 2 is live; ≤ 1 successful authorization at the network |
| C21 | Capture refused: (a) final refusal; (b) technical refusal, recaptured by the network after 2 min; (c) fault `capture_after_cancel` | (a) authorization voided, seats back on sale or listing cancelled, no tickets, fan not charged; (b) order completed once, seats never released; (c) no state change, `CAPTURE_AFTER_CANCEL` exception, M7a batch unreconciled until resolved |
| C22 | `dispute.created` for a payer whose ticket is listed: while `ACTIVE`, while `LOCKED` by a buyer who then authorizes, and while `SELLING` (capture succeeds; capture refused) | `ACTIVE`/`LOCKED`: dispute wins, the buyer's authorization is voided; `SELLING`: the dispute waits, then counts the ticket as resold (capture succeeded) or revokes it (capture refused); never a revoked ticket with a transfer; no deadlock |

### 10.3 Matrix

| Invariant | Property / model | Concurrency | Chaos |
| --- | --- | --- | --- |
| M1 | ✔ | CC1, CC11 | C6, C12, C13 |
| M2 | ✔ | CC1, CC5, CC6, CC11 | C6, C12, C13 |
| M3 | integration + tamper | — | — |
| M4 | ✔ | CC5, CC6 | C10, C13, C15, C18 |
| M5 | ✔ | CC2 | C1, C2, C5, C10, C12–C14, C17, C20, C21 |
| M6 | ✔ | CC2 | C1, C2, C9, C12, C14, C17, C20 |
| M7 | unit | — | C8, C21 |
| W1 | ✔ (Pay model) | — | C15, C16 |
| W2 | ✔ | CC8 | — |
| W3 | ✔ | CC9, CC10 | C15 |
| I1 | ✔ | CC3, CC7 | C3, C7, C11, C21 |
| I2 | ✔ | CC3, CC7 | C3, C7, C13, C21 |
| I3 | ✔ | CC4, CC12 | C13, C22 |
| I4 | ✔ (both models) | — | C2, C4, C6, C12–C14, C19–C21 |
| I5 | ✔ | CC12 | C13, C18, C19, C22 |
| X1 | ✔ | CC2 | C1, C4, C6, C12–C14, C21 |
| X2 | ✔ | CC12 | C13, C18, C19, C21, C22 |

## 11. Violation response policy

| Where | Response |
| --- | --- |
| Any automated test | Fail; attach the auditor report |
| Pay, W1 or floor < 0 | SEV-1; top-ups, sweeps and merchant settlements halt |
| Pay, M1–M6, W2, W3 | SEV-1; merchant settlements and sweeps halt |
| Tickets, M1–M6 | SEV-1; organizer and seller payouts halt |
| Either, M7 | Finance ticket; the file stays unreconciled until resolved |
| Tickets, I1–I5, X1, X2 | Page; admission and resale purchases freeze for the event |
| S1–S18 | Warn or page per §8 |

The auditor never writes; kill switches are applied by the owning components from the auditor's `/status`.

## 12. Change control

- Codes are stable and never reused; retired codes stay with status "retired".
- Adding or changing an invariant requires, in one change: an ADR, this document, a prevention mechanism (or a justification for detection only), an auditor check, and a test that fails without it.
- Relaxing an invariant requires an ADR with the business reason.
- History: v2.0 added seats and tickets to I1, I3, M5, X1; v3.0 added W1–W3 and M7a–M7c; v4.0 restored card payments and disputes in M4, M5 and X1, and added I5 and X2 for resale ownership; v4.1 allowed several card attempts per intent (M5), moved resale ownership into the capture transaction (X2), replaced the manual capture-failure exception by automatic compensation (X1), and stopped unwinding resales on refunds (I5, X2(c)), with C20–C22.
