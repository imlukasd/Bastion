# Bastion — Product Requirements Document

| Field | Value |
| --- | --- |
| Document | Product Requirements Document (PRD) |
| Status | Draft v4.1, for owner review |
| Owner | Lukas |
| Last updated | 2026-10-26 |
| Supersedes | PRD v4.0, v3.0, v2.0, v1.0, `Overview.md` and the earlier `flashwallet` proposal |
| Related | [Architecture](02-architecture.md) · [Technical Design](03-technical-design.md) · [Invariants](04-invariants.md) |

Requirement keywords **MUST**, **SHOULD** and **MAY** follow RFC 2119.

---

## 1. Summary

Bastion is two products of equal depth that together form the complete money path of a high-demand concert on-sale:

| Product | What it is | Real-world analogue |
| --- | --- | --- |
| **Bastion Pay** | An **e-wallet and payment core**: wallets linked to bank accounts, cash-in and cash-out, P2P transfers, transaction limits, PIN and OTP, a Payment Intent API that accepts **wallet and card** payments, card disputes, merchant settlement, disbursements, safeguarding of customer funds, and reconciliation with the bank and the card network | A Vietnamese e-wallet and payment gateway such as MoMo or ZaloPay |
| **Bastion Tickets** | A **concert ticketing platform** built for on-sale surges: virtual waiting room, seat reservations with a countdown, ticket issuance, an **official resale marketplace**, disputes, organizer and seller payouts, event cancellation | A ticketing site such as Ticketbox or Ticketmaster |

Bastion Tickets is **Bastion Pay's first merchant**. A fan buys tickets on Tickets and pays with their Bastion Pay wallet or a card through Bastion Pay, exactly as a ticketing site integrates an e-wallet gateway in real life:

> **Tickets:** waiting room → seat reservation with a countdown → checkout creates a Payment Intent
> **Pay:** fan confirms with PIN (wallet hold) or card (network authorization) → **Tickets:** commit seats → capture → tickets issued
> **Tickets:** fans resell tickets at face value or less; ownership moves atomically with the payment's capture; sellers are paid through Pay after the show
> **Pay:** merchant settlement → **Tickets:** organizer payouts after the show
> **Both:** disputes, refunds, cancellation, and reconciliation with the bank, the card network and each other

Five properties set Bastion apart:

1. **Two companies, two sets of books, one hard boundary.** Pay and Tickets have separate databases and ledgers and talk only through a merchant API and signed webhooks, so every integration discipline of real payments shows up on both sides.
2. **Customer money is safeguarded, not just stored.** Bastion Pay keeps an escrow account whose balance must cover every wallet balance, and proves it daily against the bank statement.
3. **Money, seats and tickets move in two phases.** Wallet holds and card authorizations come first; seats or the resale listing are committed; then money is captured, and tickets are issued or change owner in the same step. A failed purchase is **voided**, never refunded. A ticket is valid if and only if it is paid for, and has exactly one owner.
4. **Correctness is proven, not claimed.** Seventeen invariants are enforced by database constraints and checked continuously by an independent auditor and by property-based, chaos and load tests.
5. **Every major design decision traces back to a published engineering lesson** from Stripe, Amazon, Airbnb, Shopify, Uber, Alibaba, TigerBeetle and the ticketing industry, and Bastion Pay follows the Vietnamese e-wallet rules (§2.2). See [Architecture §4](02-architecture.md#4-prior-art).

The scope fits one engineer over about 22 weeks: Go, PostgreSQL and Redis, in one repository. Every external party (card network, partner bank) is a simulator with configurable faults.

## 2. Background

### 2.1 Problem

**E-wallets and payment gateways** hold and move other people's money. Their failures are silent until they are catastrophic:

| Failure mode | Impact |
| --- | --- |
| Double spending: concurrent payments, transfers and withdrawals from one wallet | Negative balances; the provider pays out money it does not have |
| Transaction limits exceeded under concurrency | Regulatory breach |
| Customer funds not fully covered by the escrow account | Regulatory breach; customers lose money if the provider fails |
| A bank or card-network call times out after the money moved | Double credits, double authorizations, lost cash-ins, double withdrawals |
| Card authorizations that expire, captures that fail, voids that never reach the network | Customers' cards held for days; merchants unpaid |
| Chargebacks after the money was spent or paid out | Losses; negative wallets; merchants in debt |
| Webhooks to merchants lost or duplicated | Merchants fulfil twice or never |
| Ledger that does not match the bank statement | Unexplained shortfalls nobody notices until an audit |

**Ticket on-sales** squeeze months of demand into minutes. In November 2022 Ticketmaster reported 3.5 billion system requests for the Taylor Swift Eras Tour presale, about four times its previous peak, and had to slow sales to keep its systems up ([Ticketmaster](https://business.ticketmaster.com/press-release/taylor-swift-the-eras-tour-onsale-explained/)):

| Failure mode | Impact |
| --- | --- |
| Overselling a zone, or selling the same seat twice | Two fans with one seat at the gate |
| Charging a fan who gets no ticket, or issuing a ticket nobody paid for | Refunds and angry fans, or silent revenue loss |
| Site meltdown at on-sale time | The whole platform goes down |
| Unfair queues and bots | Fast connections and bots win ([Queue-it](https://www.queue-it.com/blog/first-in-first-out-randomization)) |
| Scalping and resale fraud | Tickets resold above face value; the same ticket sold twice; buyers holding invalid copies |
| Disputes on ticket payments | Tickets used after the money was taken back; organizers and sellers paid for disputed sales |
| Event cancellation | Thousands of refunds at once, through primary sales and resales, possibly after money was paid out |

Bastion puts both problems on one money path. The on-sale burst lands on the payment gateway, and every resale moves both ownership and money between two fans.

### 2.2 Regulatory model (Bastion Pay)

Bastion Pay follows the Vietnamese rules for e-wallet providers as summarized by secondary sources. The official texts must be checked before any rule is relied on outside this project.

| Rule | Source (as summarized) | Modeled as |
| --- | --- | --- |
| A wallet may be used only once it is linked to a bank account or debit card of the holder | Decree 52/2024/ND-CP | `ACTIVE` only after a verified link (BR-P02) |
| Escrow (payment-guarantee) balances must not be lower than the total balance of issued wallets | Decree 52/2024/ND-CP | Invariant W1, proven daily against the bank statement |
| Transfers and payments through one customer's personal wallets at one provider are capped at 100 million VND per month | Circular 40/2024/TT-NHNN, Art. 26, as amended by Circular 41/2025/TT-NHNN | Invariant W2 (BR-P05) |
| No cash top-ups or withdrawals, no credit, no interest on wallet balances (from 5 Nov 2025) | Circular 41/2025/TT-NHNN | Bank-only cash-in/out; wallets never negative; no interest (BR-P03, BR-P04) |

Card payments processed by Bastion Pay as a gateway are not e-wallet transactions: they do not count toward wallet limits, and card money is not e-wallet money until the card network settles it into the escrow account (BR-P13).

### 2.3 Balance between the products

The two products are designed to teach different, equally deep lessons:

| Dimension | Bastion Pay | Bastion Tickets |
| --- | --- | --- |
| Race conditions | Double spend, limits at midnight, PIN brute force, freeze vs. debit, opposite transfers, hot merchant account | Oversell, seat pick vs. best available, per-fan quota, countdown vs. payment, two buyers on one resale listing, unlist vs. purchase |
| Load and fairness | Confirmation and capture bursts during an on-sale | 100,000-fan queue, shuffled pre-queue, four-layer admission funnel |
| Ledger | Customer wallets, merchant incoming/pending/available/debt, safeguarding, card float | Organizer held/available, seller payables, commission, service and resale fees, MDR and dispute costs |
| Two-phase money | Wallet holds; card attempts, authorize → capture → void, capture refusals | Reserve → commit → capture; listing lock → sale commit → capture with transfer |
| External integrations | Bank (verification, debits, transfers, statements); card network (authorize, capture, void, refund, charge, disputes, settlement); outgoing merchant webhooks | Bastion Pay (Payment Intents, refunds, disputes, funding, disbursements); bank (organizer payouts, funding transfers) |
| Unknown outcomes | Every bank and card call | Every Pay and bank call |
| Reconciliation | Escrow statements, card settlement files, safeguarding proof | Pay settlement reports, Tickets' bank statement, cross-company agreement |
| Disputes | Card disputes: merchant debit, merchant debt, dispute fees, evidence | Ticket revocation, organizer and seller recovery, credit losses |
| Domain-specific depth | Safeguarding, limits, strong customer authentication, webhook delivery | Waiting-room fairness, seat maps, ticket ownership and resale, cancellation across resale chains |

### 2.4 Purpose

Bastion is a single portfolio project that shows payments and high-concurrency engineering competence. The definition of done is **not** "the API works". It is: **all 17 invariants still hold after both systems are overloaded, have processes killed, receive wrong answers from the bank and the card network, lose the network between them, face disputes and resale races, and have an event cancelled mid on-sale** (§11).

### 2.5 History

| Version | Scope | Why it changed |
| --- | --- | --- |
| `Overview.md`, 1.0 | Marketplace flash sale with a wallet as a payment method | — |
| 2.0 | Concert ticketing | Unique seats, countdowns, tickets and cancellations make the concurrency problems harder |
| 3.0 | Bastion Pay + Bastion Tickets | The e-wallet became a product of its own |
| 4.0 | Both products at equal depth | Card two-phase payments and disputes restored (now through Bastion Pay); official resale marketplace and disbursements added; Tickets' design restored to full detail |
| 4.1 | Review fixes | Several card attempts per intent; resale ownership moves with the capture; refused captures compensated automatically; cancellation refunds independent of each other, with no ownership unwinding |

## 3. Goals and non-goals

### 3.1 Goals

| ID | Goal | Measured by |
| --- | --- | --- |
| G1 | Never lose, create or double-move money, in either company | M1–M6 violations = 0 |
| G2 | Customer funds are always fully safeguarded | W1 violations = 0 |
| G3 | Wallet rules hold under any concurrency | M4, W2, W3 violations = 0 |
| G4 | Every external record reconciles, or every difference is explained | M7 violations = 0 |
| G5 | Never oversell a zone or double-book a seat | I1 violations = 0 |
| G6 | A ticket is valid if and only if it is paid for, and has exactly one owner | X1, I5 violations = 0 |
| G7 | A resale moves ownership and money together, or neither | X2 violations = 0 |
| G8 | Survive on-sale surges by rejecting early and fairly | SLOs in §10.2; fairness metric in §11 |
| G9 | Recover automatically from crashes, partitions and unknown outcomes | Every saga reaches a terminal state within the recovery SLO |
| G10 | Cancel an event safely, including resold tickets | Every buyer refunded exactly once (C13, S13) |
| G11 | Every decision can be explained | Each decision has an architecture decision record |

### 3.2 Non-goals (MVP)

- **Bastion Pay:** QR payments at physical stores, bill payments, recurring payments, 3-D Secure flows, multi-currency, interest or credit, real KYC providers and biometrics, real SMS delivery, AML case management beyond freezing.
- **Bastion Tickets:** gate check-in apps, ticket gifting without payment, resale above face value, presale lotteries, guaranteed adjacent seats, dynamic pricing, promotions, taxes and e-invoices.
- **Both:** real bank, card-network or identity integrations; PCI scope (Bastion Pay only sees card tokens); multi-region deployment; user interfaces beyond minimal static pages.

## 4. Users and stakeholders

| Persona | Product | Needs | Key journeys |
| --- | --- | --- | --- |
| **Customer** (a fan) | Pay | A wallet that never loses money, instant transfers, clear limits, protection against stolen credentials | P1–P6 |
| **Merchant** (Bastion Tickets) | Pay | Accept wallet and card payments, refund, handle disputes, pay out to customers, get settled daily with a report | P7–P10, P13 |
| **Pay compliance, risk and finance** | Pay | Safeguarding proof, reconciliation, freezing risky wallets, managing disputes | P11, P12 |
| **Fan** | Tickets | Fair access, a clear countdown, never charged without a ticket, a safe place to resell | T1, T8 |
| **Organizer** | Tickets | Set up events, refund tickets, get paid after the show | T2, T3, T5 |
| **Reseller** (a fan) | Tickets | List a ticket at face value or less, get paid after the show | T8, T10 |
| **Tickets ops and finance** | Tickets | Run on-sales, cancel events, handle disputes, reconcile Bastion Pay | T4, T6, T7, T9 |
| **Platform engineer (SRE)** | Both | See health, recover from incidents, prove correctness | All |

External systems, all simulated:

| System | Role |
| --- | --- |
| **Card network / PSP** (`pspsim`) | Card tokens; authorize, capture, void, refund; one-step charges for top-ups; disputes and evidence; webhooks; daily settlement files |
| **Partner bank** (`banksim`) | Holds every account in the simulation with balances; holder verification, direct debits, credit transfers with returns, webhooks, daily statements |

## 5. Scope

### 5.1 Reference scenario

| Field | Value |
| --- | --- |
| Bastion Pay | 60,000 customers (50,000 in laptop load tests) with verified linked bank accounts; half also hold test card tokens |
| Merchant | Bastion Tickets: MDR 150 bps for wallet, 250 bps for card; dispute fee 50,000 VND; refund credit limit 50,000,000 VND; settlement reserve 200,000,000 VND |
| Event | *Bastion Live*, one night at a 10,000-capacity stadium |
| Zones | `GA` 4,000 · `VIP` 1,000 · `CAT1` 2,500 · `CAT2` 2,500 (seated except `GA`) |
| Prices | `GA` 1,499,000 · `VIP` 5,000,000 · `CAT1` 3,000,000 · `CAT2` 2,000,000 VND; service fee 30,000 VND per ticket |
| Per-fan limit | 4 tickets per event, including resale purchases |
| Demand | 100,000 fans in the queue; after sell-out, 10% of tickets are listed for resale |

### 5.2 Capability map

| Capability | Product | Phase |
| --- | --- | --- |
| Ledger, idempotency, FSM and outbox kits | shared | 1 |
| Customers, PIN, OTP, bank linking, wallet status, cash-in, cash-out, P2P, limits, safeguarding | Pay | 2 |
| Payment Intents (wallet and card), card two-phase flows, refunds, disputes, webhooks, merchant incoming/pending/balance/debt, settlement, funding, disbursements, card top-ups, PSP reconciliation | Pay | 3 |
| Events, seat maps, waiting room, reservations, checkout through Pay, asynchronous saga, ticket issuance | Tickets | 4 |
| Fees, refunds, disputes, event cancellation with pre-funding, organizer funds release and payouts, Pay settlement reconciliation | Tickets | 5 |
| Official resale marketplace, ownership transfer, seller payouts through Pay disbursements, cancellation refunds for resold tickets | Tickets | 6 |
| Continuous auditor (per company and cross-company) | both | 1 → 7 |
| Load, chaos and benchmark suite | both | 7 |
| Dashboards, runbooks, ADR files | both | 8 |

### 5.3 Out of scope

Everything in §3.2, plus §15 (future extensions).

## 6. User journeys

### Part A — Bastion Pay

#### P1 — Customer opens a wallet

The customer registers with a phone number and sets a 6-digit PIN. The wallet is `PENDING_VERIFICATION`. The customer links a bank account: the bank verifies that the account belongs to the same person, and a one-time password (OTP) confirms the link. The wallet becomes `ACTIVE`.

#### P2 — Customer cashes in from the linked bank

Bastion Pay debits the linked account through the bank's direct-debit API into its escrow account and credits the wallet **only after the bank confirms**. If the bank does not answer in time, Bastion Pay asks again with the same reference; the customer is credited exactly once.

#### P3 — Customer tops up with a card

The card network charges the card and Bastion Pay credits the wallet at once, although the money reaches the escrow account only at the next card settlement. Such top-ups are limited by Bastion Pay's safeguarding buffer. A later chargeback takes the money back from the wallet; whatever was already spent becomes a debt and the wallet is frozen.

#### P4 — Customer sends money to another customer

Instant, atomic, confirmed with PIN; counts toward the sender's limits.

#### P5 — Customer cashes out to the linked bank

Confirmed with PIN and OTP. The amount is held immediately and released only when the bank reports the transfer paid; a failure returns it; a later bank return re-credits it.

#### P6 — Customer pays a merchant

A merchant creates a Payment Intent. The customer opens it and chooses:

- **Wallet:** confirms with PIN; Bastion Pay checks status, balance and limits and **holds** the amount.
- **Card:** enters a card token; Bastion Pay asks the card network to **authorize** the amount.

The merchant then captures (the hold is posted, or the authorization captured) or cancels (the hold released, or the authorization voided). A failed confirmation (wrong PIN, low balance, limit, card declined) leaves the intent open for another attempt, with the wallet or another card, until it expires; only one attempt runs at a time. If the card network refuses the capture for good, Bastion Pay voids the authorization and tells the merchant the payment was cancelled.

#### P7 — Merchant integrates

Bastion Pay onboards Bastion Tickets with API keys, a webhook endpoint and signing secret, MDR per payment method, a settlement account, a refund credit limit and a settlement reserve. Every call carries an `Idempotency-Key`; webhooks are signed and retried for up to 3 days.

#### P8 — Merchant refunds

Wallet refunds credit the wallet at once. Card refunds go through the card network and complete asynchronously. Both are paid from the merchant's balance, which may go negative only down to the refund credit limit. Before a large refund wave the merchant pre-funds its balance by bank transfer.

#### P9 — Merchant is settled

Every day Bastion Pay transfers the merchant's available balance above its settlement reserve to the merchant's bank account, with a report of every capture, fee, refund, dispute, disbursement and funding behind it. Card money becomes available only after the card network has settled it.

#### P10 — Merchant pays out to a customer (disbursement)

The merchant sends money from its balance to a customer's wallet by phone number (for example, a ticket reseller's proceeds). Disbursements never use the refund credit limit: the balance must cover them.

#### P11 — Compliance proves safeguarding

Every day Bastion Pay imports the escrow statement and the card settlement file, matches every line, proves that the escrow balance covers every wallet balance, and records every difference as an exception with a reason.

#### P12 — Risk ops freezes a wallet

From the moment of the freeze, no customer-initiated debit can succeed, even if it was racing the freeze. Credits still arrive.

#### P13 — A cardholder disputes a merchant payment

The card network reports a dispute on a card payment and withdraws the funds. Bastion Pay debits the merchant's balance for the amount plus a dispute fee (recording merchant debt if the balance is not enough), notifies the merchant, forwards the merchant's evidence, and reverses the debit if the dispute is won.

### Part B — Bastion Tickets

#### T1 — Fan buys tickets in a high-demand on-sale

1. Join the pre-queue (30 minutes before the on-sale) or the queue; receive a signed queue ticket.
2. At on-sale time the pre-queue is randomized; later fans are first-in-first-out; poll timing never changes a position.
3. Exchange the queue ticket for an admission token (10 minutes).
4. Choose tickets (general admission, seat pick or best available). Tickets reserves them for **8 minutes**.
5. Check out: Tickets guarantees at least 4 minutes on the countdown and creates a Payment Intent that must be confirmed 30 s before the countdown ends.
6. Confirm in Bastion Pay with the wallet (PIN) or a card.
7. Tickets commits the seats, captures the payment and issues the tickets.

| Case | Outcome for the fan |
| --- | --- |
| Zone sold out, or per-fan limit reached | Immediate `409`; nothing is held |
| A picked seat was taken a moment earlier | `409 seats-unavailable` with the seats |
| Wallet balance low, wrong PIN, limit reached, card declined | The seats stay reserved; the fan may try again, with the wallet or another card, before the deadline |
| The card network refuses the capture | Final refusal: the authorization is voided, the seats go back on sale, nothing is charged. Temporary refusal retried by the network: the order stays "processing" until the capture succeeds |
| The fan never confirms | The intent expires; the countdown ends; nothing is charged |
| The countdown ends just as the payment is authorized | Seats cannot be committed; the hold is released or the authorization voided; nothing is charged |
| Bastion Pay or the card network is slow | Checkout fails fast or the order shows "processing"; the reservation is kept |

#### T2 — Organizer sets up an event

Draft event, zones, seat map, per-fan limit; ops reviews, sets the service fee and publishes. Published zones, prices and seats are immutable except for zone suspension.

#### T3 — Organizer refunds tickets

Tickets asks Bastion Pay to refund face value + service fee for specific valid tickets, voids them, debits the organizer's net share and reverses the platform's commission and fee. Bastion Pay's MDR stays a Tickets cost. A ticket is never refunded twice.

#### T4 — Ops cancels an event

Admission stops; reservations and open listings are released; open Payment Intents are cancelled; every ticket is voided. **Every payment is refunded independently and in any order:** each resale buyer from the held proceeds of the sale they bought from, and each primary buyer from the organizer's held funds. Ownership is not unwound; for a chain A → B → C, every fan ends at zero. Tickets pre-funds its Bastion Pay balance when needed.

#### T5 — Organizer gets paid after the show

Captured funds stay **held** until the show has ended plus a payout delay (3 days), then become **available** and are paid out daily by bank transfer.

#### T6 — Ops runs an on-sale

Live dashboard of the queue, admission, reservations, confirmations at Bastion Pay, seats and invariants. An inventory or ticket violation freezes admission automatically.

#### T7 — Finance reconciles Bastion Pay settlements

Every report line (capture, fee, refund, dispute, disbursement, funding) is matched to Tickets' records, and the bank credit is matched to the report. Differences become exceptions.

#### T8 — Fan resells a ticket

1. A ticket holder lists a valid ticket at a price **no higher than its face value**, giving the Bastion Pay phone number that should receive the proceeds.
2. The listing appears on the event's resale page. The seller can withdraw it until a buyer locks it.
3. A buyer clicks "buy": the listing is **locked** for 8 minutes for that buyer only. Many buyers may click at once; exactly one gets the lock.
4. The buyer checks out and pays through Bastion Pay, exactly as in T1.
5. Once the payment is authorized, the sale is **committed**: nobody else can buy the ticket and the seller can no longer withdraw it, but nobody can use it either. Tickets captures the payment and, in the same transaction that records the capture, **transfers ownership**: the ticket gets a new owner and a new QR version, so the seller's copy stops working at once.
6. The seller's proceeds (price minus a 5% resale fee) are **held until after the show** and paid to their wallet by Bastion Pay disbursement.

| Case | Outcome |
| --- | --- |
| Another buyer locked the listing first | `409 listing-unavailable` |
| The seller withdraws while a buyer holds the lock | `409 listing-locked`; the seller waits for the lock to end |
| The lock ends just as the payment is authorized | Ownership is not transferred; the authorization is voided |
| The card network refuses the capture | The listing is cancelled, the ticket stays with the seller (who may list it again), the authorization is voided |
| The buyer would exceed the per-fan limit | `409 purchase-limit-reached` |

#### T9 — A ticket payment is disputed

Bastion Pay reports a dispute on a card payment. Tickets **revokes** the order's tickets that the payer still owns, reverses the platform's fees, and recovers the net amount from the organizer (primary sale) or the seller's held proceeds (resale). If the payer had already resold the ticket, the buyer keeps it, and the loss is recovered from the payer's own resale proceeds first. If the payer is selling it at that very moment, the dispute is applied right after the sale completes or fails. A won dispute reinstates the tickets; a lost one voids them, or, for a resale purchase, returns the ticket to its seller.

#### T10 — Seller gets paid after the show

After the show and the payout delay, Tickets asks Bastion Pay to disburse each seller's held proceeds to their wallet.

## 7. Functional requirements

Priority: **P0** = MVP must-have, **P1** = should-have, **P2** = extension.

### Part A — Bastion Pay

#### 7.1 Customers, authentication and wallets (FR-PCU)

| ID | Requirement | Pri |
| --- | --- | --- |
| FR-PCU-01 | Customers MUST register with a unique phone number; the wallet starts `PENDING_VERIFICATION` and can neither receive nor send money. | P0 |
| FR-PCU-02 | A 6-digit PIN MUST be stored only as an Argon2id hash and required for every customer-initiated debit. | P0 |
| FR-PCU-03 | Five consecutive wrong PINs MUST lock debits for 30 minutes; the counter MUST be exact under concurrent attempts. | P0 |
| FR-PCU-04 | Linking a bank account and cashing out MUST require an OTP (5 minutes, 3 attempts, single use). | P0 |
| FR-PCU-05 | Linking MUST verify with the bank that the account holder is the wallet holder; the wallet becomes `ACTIVE` only after the first verified link. | P0 |
| FR-PCU-06 | Ops MUST be able to freeze, unfreeze and close wallets. A frozen wallet MUST reject every customer-initiated debit; credits are accepted. | P0 |
| FR-PCU-07 | Customers MUST see balance, available balance, holds, remaining limits and a statement. | P0 |

#### 7.2 Funding (FR-PFN)

| ID | Requirement | Pri |
| --- | --- | --- |
| FR-PFN-01 | Cash-in MUST debit the verified linked bank account into escrow and credit the wallet only after the bank confirms. | P0 |
| FR-PFN-02 | Card top-ups MUST charge a card token through the card network and credit the wallet on success, within the safeguarding buffer. | P0 |
| FR-PFN-03 | Cash-out MUST go only to a verified linked account of the holder; the amount is held at creation, settled when paid, released on failure, re-credited on return. | P0 |
| FR-PFN-04 | Cash or credit funding of wallets MUST NOT exist; balances MUST NOT earn interest. | P0 |
| FR-PFN-05 | Unknown outcomes of bank and card calls MUST be recorded as explicit states and resolved by lookup with the same key. | P0 |
| FR-PFN-06 | A chargeback on a top-up MUST take the amount back from the wallet; any shortfall becomes customer debt, the wallet is frozen, and later cash-ins repay the debt first. | P1 |

#### 7.3 Transfers and limits (FR-P2P, FR-PLM)

| ID | Requirement | Pri |
| --- | --- | --- |
| FR-P2P-01 | `ACTIVE` customers MUST be able to send money to other `ACTIVE` customers by phone number, confirmed by PIN, atomically and instantly. | P0 |
| FR-P2P-02 | Concurrent transfers in opposite directions MUST NOT deadlock or fail. | P0 |
| FR-PLM-01 | Each customer MUST have daily and monthly limits on P2P sends and wallet payments, in the Asia/Ho_Chi_Minh calendar. Card payments and incoming money do not count. | P0 |
| FR-PLM-02 | A wallet payment MUST reserve limit when authorized, convert it when captured and release it when cancelled. | P0 |
| FR-PLM-03 | Limits MUST never be exceeded, under any concurrency, across day and month boundaries. | P0 |

#### 7.4 Payment Intents (FR-PPI)

| ID | Requirement | Pri |
| --- | --- | --- |
| FR-PPI-01 | A merchant MUST create a Payment Intent with amount, currency, its own reference, allowed payment methods (`wallet`, `card`), a confirmation deadline and manual capture. At most one non-cancelled intent per merchant reference. | P0 |
| FR-PPI-02 | Wallet confirmation MUST check wallet status, PIN, balance and limits, then place a two-phase hold. | P0 |
| FR-PPI-03 | Card confirmation MUST authorize the amount with the card network. A timeout MUST NOT lead to a second authorization: the outcome is resolved by lookup with the same key. | P0 |
| FR-PPI-04 | A failed confirmation (any method) MUST leave the intent open for another attempt, including with another card; at most one card attempt may be in flight, and a stale or duplicate message about an earlier attempt MUST NOT change the intent; five failed confirmations cancel it. | P0 |
| FR-PPI-05 | A merchant MUST capture once, for at most the authorized amount. A card capture MUST NOT start within 1 hour of the authorization's expiry. An authorization not captured within its capture window (15 min by default) MUST be cancelled automatically (hold released or authorization voided). A capture refused for a final reason MUST be followed by an automatic void; a refusal the network will retry MUST keep the intent processing; the intent becomes cancelled only once the authorization can no longer be captured. | P0 |
| FR-PPI-06 | A merchant MUST be able to cancel an intent before capture. Confirmation, capture, cancellation and expiry racing each other MUST have exactly one winner. | P0 |
| FR-PPI-07 | Merchants MUST be able to refund a captured intent fully or partly, any number of times, never above the captured amount minus disputed amounts. Wallet refunds are instant; card refunds complete asynchronously. | P0 |
| FR-PPI-08 | MDR MUST be charged at capture per payment method and MUST NOT be returned on refunds or disputes. | P0 |
| FR-PPI-09 | Every object MUST be retrievable by the idempotency key that created it. | P0 |

#### 7.5 Disputes (FR-PDS)

| ID | Requirement | Pri |
| --- | --- | --- |
| FR-PDS-01 | A card dispute on a captured intent MUST debit the merchant's balance for the disputed amount plus a dispute fee; any part beyond the refund credit limit MUST be recorded as merchant debt, repaid from future captures. | P0 |
| FR-PDS-02 | The merchant MUST be notified and MUST be able to submit evidence until the evidence deadline; Bastion Pay forwards it to the card network. | P1 |
| FR-PDS-03 | A won dispute MUST reverse the amount (not the fee); a lost one is final. | P0 |
| FR-PDS-04 | Refunded + disputed MUST NOT exceed captured for any intent. | P0 |

#### 7.6 Merchants, webhooks, settlement and disbursements (FR-PMR, FR-PWH)

| ID | Requirement | Pri |
| --- | --- | --- |
| FR-PMR-01 | Wallet captures MUST become part of the merchant's balance within seconds without a single hot row limiting throughput. | P0 |
| FR-PMR-02 | Card captures MUST stay pending until the card network settles them, then become available. | P0 |
| FR-PMR-03 | Every day, the merchant's available balance above its settlement reserve (≥ the minimum) MUST be transferred to its bank account with a settlement report. | P0 |
| FR-PMR-04 | A merchant balance MUST NOT go below `−refund_credit_limit`; forced debits beyond it become merchant debt. | P0 |
| FR-PMR-05 | Merchants MUST be able to pre-fund their balance through a bank transfer carrying a Bastion Pay reference. | P0 |
| FR-PMR-06 | Merchants MUST be able to disburse money to a customer wallet by phone number; disbursements MUST NOT use credit; a recipient lookup returns the masked holder name. | P0 |
| FR-PWH-01 | Every merchant-visible change MUST produce a signed webhook event in the same transaction, delivered with exponential backoff for up to 3 days; order is not guaranteed; every event has a unique id. | P0 |

#### 7.7 Safeguarding and reconciliation (FR-PSG)

| ID | Requirement | Pri |
| --- | --- | --- |
| FR-PSG-01 | Customer and merchant e-wallet money MUST be held in a dedicated escrow account, separate from the operating account. | P0 |
| FR-PSG-02 | Every day, Bastion Pay MUST match every escrow statement line to the ledger and prove that the bank-confirmed closing balance covers all e-wallet balances at the cut-off. | P0 |
| FR-PSG-03 | Operations that let e-wallet balances run ahead of escrow money (card top-ups, revenue sweeps) MUST be refused when the conservative safeguarding surplus is insufficient. | P0 |
| FR-PSG-04 | Card settlement files MUST be matched to top-ups, intent captures, card refunds and disputes; matched intent captures release the merchant's pending card funds. | P0 |
| FR-PSG-05 | Every difference MUST become an exception with a reason, resolved by ops with an audit trail. | P0 |

### Part B — Bastion Tickets

#### 7.8 Events, waiting room and reservations (FR-CAT, FR-WR, FR-RSV)

| ID | Requirement | Pri |
| --- | --- | --- |
| FR-CAT-01 | Organizers MUST create draft events with zones (`GA` or `SEATED`) and seat maps whose seat counts equal capacities; ops publishes and sets the service fee; published data is immutable except zone suspension. | P0 |
| FR-CAT-02 | Public endpoints MUST report event state, coarse availability per zone, and seat availability at most 2 s stale. | P0 |
| FR-CAT-03 | Events MUST move automatically through pre-queue, on-sale, sales closed and completed. | P0 |
| FR-WR-01 | One signed queue ticket per fan and event; pre-queue fans randomized at on-sale time; later fans FIFO; position independent of polling. | P0 |
| FR-WR-02 | Admission MUST follow measured goodput and latency, MUST NOT admit more fans than the remaining inventory can serve, and MUST freeze on an I or X violation. | P0 |
| FR-WR-03 | Reservations without a valid admission token MUST be rejected with a problem pointing to the waiting room, never a bare `429`. | P0 |
| FR-RSV-01 | Reservations MUST be all-or-nothing across items `{zone, qty}` or `{zone, seat_ids}`; seat pick holds exactly the requested seats; best available never waits on other fans' seats. | P0 |
| FR-RSV-02 | The per-fan limit MUST count held, purchased and resale-purchased tickets across the event. | P0 |
| FR-RSV-03 | Reservations expire after 8 minutes; checkout extends them so 4 minutes remain, never beyond 12 minutes in total; expired reservations are released within 5 s. | P0 |
| FR-RSV-04 | Sold-out and over-limit requests MUST be rejected on a fast path that does not touch PostgreSQL, rebuilt from PostgreSQL after loss or drift. | P0 |

#### 7.9 Checkout, tickets and disputes (FR-CO, FR-TKT, FR-DSP)

| ID | Requirement | Pri |
| --- | --- | --- |
| FR-CO-01 | Checkout MUST create (or return) one Bastion Pay Payment Intent per order, allowing wallet and card, with a confirmation deadline 30 s before the reservation ends. | P0 |
| FR-CO-02 | On authorization, Tickets MUST commit every reservation (or the resale listing), capture exactly the order total, and issue tickets or transfer ownership in the transaction that records the capture; if it cannot commit, it MUST cancel the intent. | P0 |
| FR-CO-03 | Tickets MUST learn outcomes from webhooks and, when they are late or lost, from polling; asynchronous captures (card) MUST be followed to completion. Once a capture is sent, only a terminal outcome (succeeded, or cancelled after a refused capture) changes the order. | P0 |
| FR-CO-04 | Interrupted sagas MUST be resumed or compensated automatically, including a refused capture (seats released or listing cancelled, nothing charged). A breaker on Bastion Pay fails checkouts fast; reservations are kept. | P0 |
| FR-TKT-01 | One ticket per purchased unit, issued in the transaction that records the capture; seated tickets name their seat; a seat has at most one live ticket. | P0 |
| FR-TKT-02 | Each ticket has one owner and an owner version; its QR payload is signed with a rotatable key and includes the version, so older copies stop verifying after a transfer. | P0 |
| FR-TKT-03 | Ticket status MUST follow money: `REFUND_PENDING` during a refund, `VOID` once refunded, `REVOKED` while disputed; on a cancelled event every ticket is `VOID` and refunds move money only. | P0 |
| FR-DSP-01 | On a dispute, Tickets MUST revoke the disputed tickets still owned by the payer, reverse platform fees, recover the net amount from the organizer (primary) or seller proceeds (resale), and book the rest as credit losses. | P0 |
| FR-DSP-02 | Tickets SHOULD submit evidence (order, admission and transfer records) through Bastion Pay. | P1 |
| FR-DSP-03 | A won dispute MUST reinstate tickets and reverse recoveries (not the dispute fee); a lost one voids primary tickets and returns a resale ticket to its seller (only if the buyer still owned it, and not for a cancelled event). | P0 |
| FR-DSP-04 | A dispute MUST win over a resale of the payer's ticket that has not yet been committed (the listing is cancelled and the buyer's authorization voided) and MUST wait for one that has (applied as soon as the sale completes or fails). | P0 |

#### 7.10 Refunds, cancellation and money (FR-REF, FR-MNY)

| ID | Requirement | Pri |
| --- | --- | --- |
| FR-REF-01 | Organizers or ops MUST refund specific valid tickets (face value + service fee); a ticket is refunded at most once; listed tickets must be unlisted first. | P0 |
| FR-REF-02 | Organizer-requested refunds MUST respect the organizer overdraft limit. | P0 |
| FR-REF-03 | Event cancellation MUST stop admission, cancel listings and open intents, void every ticket, and refund every primary and resale payment independently and in any order, each from money held for that sale, pre-funding Bastion Pay when needed; rate-limited and resumable. | P0 |
| FR-MNY-01 | Primary split per ticket: commission = `floor(face × bps / 10 000)`, organizer net = face − commission, platform = commission + service fee. MDR and dispute fees are platform costs. | P0 |
| FR-MNY-02 | Organizer funds MUST stay held until the show has ended + payout delay; organizer payouts run daily by bank transfer. | P0 |
| FR-MNY-03 | Tickets MUST reconcile every Bastion Pay settlement report and its own bank statement. | P0 |

#### 7.11 Resale (FR-RES)

| ID | Requirement | Pri |
| --- | --- | --- |
| FR-RES-01 | The owner of a `VALID` ticket MUST be able to list it at a price ≤ its face value, with a verified Bastion Pay payout phone, until 2 h before the show. | P0 |
| FR-RES-02 | A ticket MUST have at most one open listing; listed tickets cannot be refunded, transferred elsewhere or used for entry. | P0 |
| FR-RES-03 | Buying MUST lock the listing for one buyer for 8 minutes; concurrent buyers get exactly one winner; the seller cannot withdraw a locked listing. | P0 |
| FR-RES-04 | Resale purchases MUST count toward the buyer's per-fan limit and leave the seller's count. | P0 |
| FR-RES-05 | On authorization, the sale MUST be committed while the lock is valid (otherwise the authorization MUST be voided); ownership MUST then transfer atomically (new owner, new owner version, transfer record) in the transaction that records the capture. Until then the ticket stays the seller's and nobody can use, list or refund it; if the capture is refused, the listing is cancelled and the ticket stays with the seller. | P0 |
| FR-RES-06 | Seller proceeds = price − resale fee (5%, rounded down); the buyer pays price + service fee; proceeds are held until the show has ended + payout delay and then disbursed through Bastion Pay. | P0 |
| FR-RES-07 | On event cancellation, every resale buyer MUST be refunded from the held proceeds of the sale they bought from, independently of every other refund; ownership is not unwound. | P0 |

### Part C — Shared

| ID | Requirement | Pri |
| --- | --- | --- |
| FR-LED-01 | Each company MUST keep its own double-entry, immutable, hash-chained ledger with two-phase holds, rebuildable balances and one entry per business event. | P0 |
| FR-API-01 | Every endpoint that moves money, seats, tickets or ownership MUST require `Idempotency-Key` (`400` if missing, replay on repeat, `422` on mismatch, `409` while in progress). | P0 |
| FR-API-02 | Errors MUST use RFC 9457 problem details; APIs are versioned; every request has a server deadline. | P0 |
| FR-API-03 | Incoming webhooks MUST be signature-verified, timestamp-checked, deduplicated and applied only along legal transitions. | P0 |
| FR-MSG-01 | Domain events MUST be written to a transactional outbox; delivery at least once; consumers deduplicate. | P0 |
| FR-OPS-01 | An independent auditor MUST check all 17 invariants and the secondary checks, per company and across companies; a CLI MUST provide rebuild, verification, replay, resolution, freezing and seeding tools; ops actions are audit-logged. | P0 |
| FR-SIM-01 | `pspsim` MUST support tokens, authorize/capture/void/refund, charges, lookup by key, disputes and evidence, signed webhooks, settlement files and seeded faults. | P0 |
| FR-SIM-02 | `banksim` MUST hold accounts with balances and support verification, direct debits, transfers with returns, lookup, signed webhooks, daily statements and seeded faults. | P0 |

## 8. Business rules and defaults

### 8.1 Bastion Pay

| ID | Rule | Default |
| --- | --- | --- |
| BR-P01 | Currency VND (exponent 0); amounts `int64`. | VND |
| BR-P02 | A wallet becomes `ACTIVE` only after a bank account in the holder's name is verified and linked. | — |
| BR-P03 | Money enters a wallet only by cash-in, card top-up, P2P, refund or disbursement; it leaves only by cash-out, P2P, wallet payment or chargeback. | — |
| BR-P04 | No cash, no credit, no interest; wallets never below zero; chargeback shortfalls become customer debt. | — |
| BR-P05 | Limits on P2P sends + wallet payments per customer (Asia/Ho_Chi_Minh calendar). | 20,000,000 VND/day (product rule); 100,000,000 VND/month (§2.2) |
| BR-P06 | PIN 6 digits, 5 attempts, 30-min lock. OTP 6 digits, 5 min, 3 attempts. | — |
| BR-P07 | MDR per capture, rounded down, never returned. | Wallet 150 bps; card 250 bps |
| BR-P08 | Safeguarding buffer of Bastion Pay's own capital in escrow; card top-ups and revenue sweeps never exceed the conservative surplus. | 5,000,000,000 VND |
| BR-P09 | Intent: confirmation deadline ≤ 24 h; capture window 15 min (card ≤ 7 days); card authorization valid 7 days, no capture within 1 h of expiry; 5 failed confirmations cancel. | — |
| BR-P10 | Merchant settlement daily: available balance − settlement reserve, if ≥ the minimum. | 01:00; 100,000 VND |
| BR-P11 | Merchant balance ≥ −refund credit limit; forced debits beyond it become merchant debt, repaid first from new captures. | Per merchant |
| BR-P12 | Dispute fee charged to the merchant per card dispute, not returned when won. | 50,000 VND |
| BR-P13 | Card captures stay pending until the card network's settlement includes them, then become available. | — |
| BR-P14 | Disbursements require available balance ≥ amount (no credit) and an `ACTIVE` or `FROZEN` recipient wallet. | — |
| BR-P15 | Refunds to a closed wallet are parked as unclaimed funds. | — |

### 8.2 Bastion Tickets

| ID | Rule | Default |
| --- | --- | --- |
| BR-T01 | Fan pays face value + service fee per ticket, snapshotted at reservation. | 30,000 VND |
| BR-T02 | Commission per organizer, per ticket, rounded down. | 500 bps |
| BR-T03 | Per-fan limit per event across zones, counting held, purchased and resale-purchased tickets; refunds do not restore it. | 4 |
| BR-T04 | Countdown 8 min; checkout guarantees 4 min; total ≤ 12 min; intent deadline = countdown end − 30 s. | — |
| BR-T05 | Admission token 10 min; pre-queue 30 min. | — |
| BR-T06 | Orders are all-or-nothing and belong to one event. | — |
| BR-T07 | Refunds only by organizer or ops decision, or event cancellation; full face value + service fee; the platform absorbs MDR. | — |
| BR-T08 | Organizer funds held until `ends_at` + payout delay; no cancellation after completion. | 3 days |
| BR-T09 | Organizer overdraft limit (after funds release). | 20,000,000 VND |
| BR-T10 | Cancellation refunds rate-limited; pre-funded at Pay when needed. | 50 refunds/s |
| BR-T11 | Resale price ≤ face value; listings close 2 h before the show; buyer lock 8 min. | — |
| BR-T12 | Resale fee charged to the seller, rounded down; the buyer pays the service fee. | 500 bps |
| BR-T13 | Seller proceeds held until `ends_at` + payout delay, then disbursed through Bastion Pay. | 3 days |
| BR-T14 | In a dispute: revoke tickets still owned by the payer; recover from organizer (primary) or seller proceeds (resale); if the payer resold the ticket, recover from the payer's resale proceeds first; the rest is credit loss; dispute fee and MDR are platform costs. | — |
| BR-T15 | Refunded or revoked seats are not put back on sale in the MVP; seats released after a refused capture go back on sale like expired reservations (nobody paid for them). | — |

## 9. Correctness contract

Seventeen invariants, specified in [Invariants](04-invariants.md). M invariants apply to each company's books.

| Code | Invariant (short form) |
| --- | --- |
| M1 | Every journal entry sums to zero, per currency. |
| M2 | Every cached balance equals the sum of its postings. |
| M3 | Postings are append-only; sealed history is hash-chained. |
| M4 | Accounts stay within their bounds; outflows never exceed the available balance; disbursements never use credit. |
| M5 | No double charge: one live intent per merchant reference and one live payment per order; at most one live card attempt and one successful authorization per intent, one capture; refunded + disputed ≤ captured. |
| M6 | One idempotency key produces exactly one effect. |
| M7 | Every external record reconciles, or every difference is an exception with a reason. |
| W1 | The bank-confirmed escrow balance covers all e-wallet balances at every statement cut-off. |
| W2 | No customer exceeds a daily or monthly limit, holds included. |
| W3 | No customer-initiated debit on a wallet that is not `ACTIVE`; bank money only to and from the holder's verified accounts. |
| I1 | No oversell and no double booking. |
| I2 | Inventory is conserved; counters equal facts; Redis converges. |
| I3 | No fan exceeds the per-event limit, holds and resale purchases included. |
| I4 | State machines move only along legal edges in both products; an expired reservation or lock is never committed. |
| I5 | Every live ticket has exactly one owner and at most one open listing; ownership changes only through a completed resale (or back, one hop, after a lost resale dispute); listings never exceed face value. |
| X1 | Money, seats and tickets agree for primary sales. |
| X2 | Money and ownership agree for resales: a buyer is charged if and only if ownership moved to them, and a seller is credited if and only if it did. |

## 10. Non-functional requirements

### 10.1 Correctness and durability

| ID | Requirement |
| --- | --- |
| NFR-COR-01 | All 17 invariants MUST hold under every test condition. |
| NFR-COR-02 | PostgreSQL MUST be each company's only source of truth. |
| NFR-COR-03 | `synchronous_commit = on`; RPO 0 for acknowledged writes. |
| NFR-COR-04 | No database transaction may span both companies. |

### 10.2 Performance (initial targets, calibrated in Phase 7)

| ID | Metric | Target |
| --- | --- | --- |
| NFR-PERF-01 | Pay: P2P, p99 | ≤ 100 ms |
| NFR-PERF-02 | Pay: wallet confirmation, p99 / card confirmation (one network call), p99 | ≤ 200 ms / ≤ 1 s |
| NFR-PERF-03 | Pay: sustained confirmations / captures for one merchant | ≥ 300/s / ≥ 500/s |
| NFR-PERF-04 | Tickets: waiting room p99 / sustained | ≤ 50 ms / ≥ 5,000/s |
| NFR-PERF-05 | Tickets: reservation p99 / checkout p99 | ≤ 300 ms / ≤ 500 ms |
| NFR-PERF-06 | Tickets: resale lock p99 (500 buyers on one listing) | ≤ 300 ms |
| NFR-PERF-07 | End to end: confirmation in Pay → tickets issued or transferred, p99 | ≤ 2 s |
| NFR-PERF-08 | Deterministic outcomes (not `5xx`) under target load | ≥ 99% |
| NFR-PERF-09 | Cancelling the reference event (10,000 tickets, 1,000 resold) refunds everything | ≤ 20 min at 50 refunds/s |

### 10.3 Resilience, security, observability, maintainability

| ID | Requirement |
| --- | --- |
| NFR-RES-01 | Bounded queues, deadlines, budgeted jittered retries at one layer only; four-layer shedding at Tickets. |
| NFR-RES-02 | Each product keeps serving its own users when the other is down. |
| NFR-REC-01 | Interrupted sagas terminal within 60 s after restart; unknown outcomes alert after 15 min. |
| NFR-SEC-01 | Least-privilege roles; no application role may update or delete ledger rows; the auditor is read-only. |
| NFR-SEC-02 | PINs and OTPs only as hashes, verified in constant time, never logged; merchant API keys hashed; webhooks HMAC-signed with rotatable secrets; QR keys rotatable. |
| NFR-SEC-03 | No card numbers stored or logged; `govulncheck` in CI. |
| NFR-OBS-01 | Correlation IDs and traces across both products; invariants, safeguarding, limits, webhook backlog, unknowns, Redis drift, refund backlog, dispute counts and resale locks visible as metrics. |
| NFR-MNT-01 | The boundary between the products and between modules MUST be enforced by an architecture test; every significant decision has an ADR. |
| NFR-OPS-01 | One command starts everything and one runs the demo; benchmarks are reproducible. |

## 11. Success metrics and definition of done

| Metric | Target | Evidence |
| --- | --- | --- |
| Invariant violations (all 17) during load and chaos | 0 | Auditor report per benchmark |
| Chaos scenarios C1–C22 | All pass | CI and nightly |
| Concurrency tests CC1–CC12 | All pass under `-race` | CI |
| Double spend on one wallet; limits at midnight | Never negative; never exceeded | CC5, CC8 |
| Safeguarding during a 3-day card settlement delay | W1 holds; top-ups refused when the surplus is used | C16 |
| Card authorization timeout after success | One authorization at the network | C17 |
| Card declined, another card on the same intent; stale webhooks of the first attempt | Second attempt recorded; never two live attempts | C20 |
| Capture refused: final; retried and recaptured by the network | Final: seats back on sale, nothing charged, no manual step. Retried: tickets issued once | C21 |
| Oversell / double booking | Exactly capacity sold; one order per contested seat | CC3, CC7 |
| 500 buyers on one listing, seller unlisting at the same time | Exactly one outcome; one owner | CC12 |
| Dispute on a resold ticket | Buyer keeps the ticket; payer's proceeds recovered | C18 |
| Dispute while the payer is reselling the ticket | Before the sale commits, the dispute wins; after, it waits for the sale | C22 |
| Event cancellation with resale chains | Every payment refunded exactly once, in any order; every fan at zero | C13 |
| Waiting-room fairness | Pre-queue order uniform (χ²); 99% of later fans within one batch of arrival rank | Fairness report |
| Performance | §10.2 met or deviation documented | `docs/benchmarks/` |

## 12. Release plan

About 22 weeks at 15–20 hours per week. **A phase ends only when its invariants and chaos scenarios pass as automated tests.** Three safe stopping points: after Phase 3 (complete e-wallet and payment gateway), after Phase 4 (complete on-sale), after Phase 6 (complete project before load proof).

| Phase | Weeks | Scope | Exit gate |
| --- | --- | --- | --- |
| 1. Foundations | 1–2 | Monorepo, Compose (two databases), ledger, idempotency, FSM and outbox kits, auditor skeleton, `banksim` accounts | M1–M4, M6 properties; CC1 at ledger level |
| 2. Pay wallet core | 3–5 | Customers, PIN, OTP, linking, cash-in, cash-out, P2P, limits, freeze, surplus floor, escrow reconciliation | W1–W3; CC2, CC5, CC8–CC10; C1, C10 |
| 3. Pay merchant payments | 6–9 | Payment Intents (wallet and card), card authorize/capture/void/refund, disputes, webhooks, incoming sweep, card pending funds, settlement, funding, disbursements, card top-ups and chargebacks, PSP reconciliation | M5, M7 (Pay); CC11; C2, C9, C15, C16, C17, C20 |
| 4. Tickets on-sale | 10–13 | Events, seat maps, waiting room, inventory, Redis filter, reservations, checkout through Pay, asynchronous saga with capture-refusal compensation, tickets | I1–I4, X1; CC3, CC4, CC7; C3, C4, C6, C7, C11, C12, C14, C21 |
| 5. Tickets money | 14–15 | Fees, refunds, disputes, event cancellation with pre-funding, organizer funds release and payouts, Pay settlement reconciliation | M7 (Tickets); CC6; C5, C8 |
| 6. Resale | 16–18 | Listings, locks, sale commit and ownership transfer, resale payments, seller proceeds and disbursements, resale disputes, cancellation with resale chains | I5, X2; CC12; C13, C18, C19, C22 |
| 7. Proof under load | 19–20 | End-to-end k6 scenarios, chaos suite, hot-account benchmarks, `SERIALIZABLE` comparison | All invariants hold; reports published |
| 8. Hardening | 21–22 | Dashboards, ADR files, runbooks, README, demo | A fresh clone runs `task demo` end to end |

## 13. Assumptions, constraints and dependencies

- **Constraints:** one engineer; one laptop; no paid services; about 22 weeks.
- **Assumptions:** Stripe-like card semantics (authorize, capture once, void, refund, disputes with evidence, settlement files); a bank with verification, direct debits, transfers with returns and daily statements; one account per person in each product.
- **Dependencies:** Go ≥ 1.25, PostgreSQL ≥ 17, Redis ≥ 7.2, Docker, k6, Toxiproxy, Prometheus, Grafana.

## 14. Risks and mitigations

| Risk | Likelihood | Impact | Mitigation |
| --- | --- | --- | --- |
| Scope: two deep products | High | Schedule | Three stopping points; strict gates; extensions deferred (§15) |
| Regulatory details misread | Medium | Credibility | Rules as configuration; sources cited with a "verify" note |
| Hot merchant account at Pay | High | Throughput | Sharded incoming + swept bounded balance; CC11 |
| Hot zone rows at Tickets | Medium | Throughput | Redis filter, inventory-aware admission, narrow rows |
| Resale locks hoarded by bots | Medium | Fairness | Per-fan limit counts locks; lock TTL; rate limits per fan |
| Webhook storms between the products | Medium | Latency | Delivery queue with backoff; receivers acknowledge first |
| Deadlocks | Medium | Correctness | One lock order per product (Technical Design §18.1, §28.1); CC1, CC7, CC12 |
| Simulator fidelity | Medium | Credibility | Documented semantics and deviations |
| Laptop results do not generalize | High | Expectations | Publish the environment with every number |

## 15. Future extensions (P2)

- **Bastion Pay:** 3-D Secure, QR at stores, bill payments, tiered KYC limits, device binding, velocity rules, multiple escrow banks, rolling reserves, automatic debit of negative merchant balances, ISO 20022 statements.
- **Bastion Tickets:** gate check-in with offline scanners and rotating QR, Verified-Fan presale lottery, gifting, adjacent best-available, per-seat Redis claims, advance payouts, resale price floors and organizer royalties.
- **Both:** TigerBeetle benchmark, stateless waiting room, `porcupine` history checking, TLA+ specification of the sagas, external anchoring of hash chains.

## 16. Glossary

| Term | Meaning |
| --- | --- |
| E-wallet | An account at Bastion Pay holding a customer's or merchant's money |
| Escrow (payment-guarantee) account | Bastion Pay's bank account for e-wallet money, separate from its operating account |
| Safeguarding surplus | Escrow balance minus all e-wallet balances |
| Linked bank account | A verified account of the wallet holder; the only source and destination of bank money |
| Limit | A cap on P2P sends and wallet payments per customer per day or month |
| Payment Intent | A merchant's request to be paid, confirmed by a customer with a wallet or a card, then captured or cancelled |
| Authorization / capture / void | Holding money / taking held money / releasing held money |
| MDR | Merchant discount rate, Bastion Pay's fee per capture |
| Pending card funds | Card captures not yet settled by the card network; not spendable by the merchant |
| Merchant debt | What a merchant owes Bastion Pay after forced debits exceeded its credit limit |
| Disbursement | A merchant's payment into a customer's wallet |
| Dispute / chargeback | A cardholder contesting a payment; the card network takes the money back |
| Event / zone / seat | A concert; a priced area (`GA` or `SEATED`); one seat |
| Queue ticket / admission token | A signed waiting-room position / a short-lived pass to reserve |
| Reservation | A time-boxed hold on tickets and seats, with a countdown |
| Ticket / owner version | The right to attend / a counter that changes on every transfer and is part of the QR payload |
| Listing / lock | A resale offer / an 8-minute exclusive right of one buyer to pay for it |
| Seller proceeds | Resale price minus resale fee, held until after the show |
| Goodput | Requests per second that reached a deterministic outcome |
| Saga | A multi-step process with persisted progress and compensation |
| Unknown outcome | A call whose result was not observed, resolved by asking again with the same key |

## 17. Change log

| Date | Version | Change |
| --- | --- | --- |
| 2026-10-14 | 1.0 | Replaces `Overview.md`. |
| 2026-10-26 | 2.0 | Concert ticketing. |
| 2026-10-26 | 3.0 | Bastion Pay and Bastion Tickets. |
| 2026-10-26 | 4.0 | Equal depth: card payments (authorize/capture/void/refund) and disputes through Bastion Pay; disbursements; official resale marketplace; I5, X2, CC12, C17–C19; Tickets' design restored to full detail; 22-week plan. |
