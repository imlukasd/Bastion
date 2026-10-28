# Bastion documentation

Bastion is two products of equal depth that together form the money path of a high-demand concert on-sale:

- **Bastion Pay**, an e-wallet and payment gateway: wallets linked to bank accounts, cash-in and cash-out, P2P, limits, PIN and OTP, Payment Intents paid by wallet or card, card disputes, merchant settlement, disbursements, and safeguarding of customer funds.
- **Bastion Tickets**, a concert ticketing platform built for on-sale surges, with an official resale marketplace, and Bastion Pay's first merchant.

Their correctness is defined by 17 invariants, enforced in the database and continuously audited.

## Reading order

| # | Document | Answers | Audience |
| --- | --- | --- | --- |
| 1 | [PRD](01-prd.md) | What are we building, for whom, under which rules, and how do we know it is done? | Everyone |
| 2 | [Architecture](02-architecture.md) | How is it structured? Why this design and not the alternatives? | Engineers, reviewers, interviewers |
| 3 | [Technical Design](03-technical-design.md) | Exactly how does each part work: schemas, SQL, Lua, APIs, jobs, tests? | Implementers |
| 4 | [Invariants](04-invariants.md) | What must always be true, and how is each rule prevented, detected and tested? | Everyone; read before writing code |

## Conventions

- These four documents are the single source of truth; when code and documents disagree, fix one of them in the same change.
- How to work in the repository: the loop step by step in [WORKFLOW.md](WORKFLOW.md), the agents' protocol in [AGENTS.md](../AGENTS.md); where the project stands in [progress/NOW.md](progress/NOW.md); which sections a task needs in [CONTEXT-MAP.md](CONTEXT-MAP.md).
- Decision records live in [Architecture §10](02-architecture.md#10-architecture-decisions); from Phase 8 they are also split into `docs/adr/`.
- Benchmark runs write reports to `docs/benchmarks/YYYY-MM-DD-<scenario>.md` (Technical Design §34.3).

| Prefix | Meaning | Defined in |
| --- | --- | --- |
| G, FR, BR-P, BR-T, NFR | Goals, requirements, business rules (Pay, Tickets), non-functional requirements | PRD |
| AD | Architecture decisions | Architecture |
| P, T | Ledger entry templates of Bastion Pay and Bastion Tickets | Technical Design §9.2, §20.2 |
| M, W, I, X | Contract invariants (money, wallet, inventory and ownership, cross) | Invariants |
| S, CC, C | Secondary checks, concurrency tests, chaos scenarios | Invariants §8, §10 |

## History

| Date | Version | Change |
| --- | --- | --- |
| 2026-10-13 | — | `Overview.md` (marketplace flash sale) |
| 2026-10-14 | 1.0 | Four documents replace `Overview.md` |
| 2026-10-26 | 2.0 | Concert ticketing |
| 2026-10-26 | 3.0 | Split into Bastion Pay and Bastion Tickets |
| 2026-10-26 | 4.0 | Equal depth: card payments and disputes through Pay, disbursements, official resale, Tickets restored to full detail |
| 2026-10-26 | 4.1 | Review fixes: card attempts per intent; resale ownership moves with the capture (`SELLING` commit point); refused captures compensated automatically; cancellation refunds independent, no ownership unwinding; C20–C22 |
