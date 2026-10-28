# Context map

Which parts of the spec to read for a task. Never read docs/01–04 in full (about 80k tokens); read the rows you need.

TD = [Technical Design](03-technical-design.md), INV = [Invariants](04-invariants.md), ARCH = [Architecture](02-architecture.md), PRD = [PRD](01-prd.md). To read a section, find its line with a heading search (for example `grep -n "^## 24\." docs/03-technical-design.md`) and read from there to the next heading of the same level.

## Always, for any change that writes data

- [ENGINEERING.md](ENGINEERING.md) for any code change, and [REVIEW.md](REVIEW.md) before handing an item over.
- TD §1 Conventions (money, identifiers, time, naming, errors) and TD §2 (layout; lock, then mutate, then post).
- ARCH §7.1 Repository and boundaries.
- INV §3 Summary: find every invariant the change can affect, then read those in full.
- The product's lock order: Pay TD §18.1, Tickets TD §28.1.

## By area

| Area | TD | INV | ARCH | PRD |
| --- | --- | --- | --- | --- |
| Repository, tooling, local stack | §2, §33, §35 | — | §7.1, §12.1, AD-15 | §12, §13 |
| Ledger kit | §3; charts §9 (Pay), §20 (Tickets) | §2.1, M1–M4, M6 | AD-08, AD-14, AD-18 | — |
| Idempotency kit | §4 | M6 | AD-13 | — |
| State machine kit | §5; Tickets graphs §21 | I4 | AD-09 | — |
| Calls across boundaries, resolvers, incoming webhooks | §6 | M5, M6 | AD-13 | — |
| Outbox and event streams | §7 | §9 (E1, E2) | AD-11 | — |
| Pay: customers, authentication, wallet status | §8, §10 | W3 | AD-30 | §7.1 |
| Pay: limits | §11 | W2 | AD-29 | §7.3 |
| Pay: funding (cash-in, cash-out, card top-ups) | §12 | W1, M4, M7 | AD-32, AD-33 | §7.2 |
| Pay: P2P transfers | §13 | M1, M4, W2, W3 | — | §7.3 |
| Pay: Payment Intents (wallet and card), disputes | §14 | M5, M6 | AD-06, AD-26, AD-27, AD-33, AD-34 | §7.4, §7.5 |
| Pay: merchants, settlement, disbursements | §15 | M4 | AD-10, AD-35 | §7.6 |
| Pay: webhook delivery | §16 | §9 (E3) | AD-31 | §7.6 |
| Pay: safeguarding and reconciliation | §17 | W1, M7 | AD-28 | §7.7 |
| Pay: concurrency and jobs | §18 | §10.1 (CC1, CC2, CC5, CC8–CC11) | AD-09, AD-17 | — |
| Tickets: data model and state machines | §19, §21 | I4 | — | — |
| Tickets: waiting room | §22 | — | AD-12 | §7.8 |
| Tickets: inventory and admission filter | §23 | I1, I2 | AD-04, AD-22 | §7.8 |
| Tickets: reservation and payment saga | §24 | I1–I4, X1, M5 | AD-05, AD-07, AD-23, AD-24 | §7.8, §7.9 |
| Tickets: refunds, disputes, cancellation | §25 | X1, X2, I5 | AD-25, AD-34, AD-38 | §7.9, §7.10 |
| Tickets: resale | §26 | I3, I5, X2 | AD-36, AD-37 | §7.11 |
| Tickets: organizer money, settlement reconciliation | §27 | M4, M7, §8 (S16, S17) | AD-25 | §7.10 |
| Tickets: concurrency and jobs | §28 | §10.1 (CC3, CC4, CC6, CC7, CC12) | AD-09, AD-17 | — |
| Flows across both products | ARCH §8.1–§8.7 | X1, X2, §8 (S16) | — | §6 |
| APIs | §29 | — | AD-16 | — |
| Simulators (`pspsim`, `banksim`) | §30 | M7 | AD-19 | — |
| Auditor | §31 | each invariant's "Detect"; §8 | AD-20 | — |
| Observability, configuration | §32, §33 | — | AD-21 | §10.3 |
| Tests, CI, chaos, load | §34 | §10 | AD-20 | §11, §12 |

User journeys are in PRD §6 (P1–P13 for Pay, T1–T10 for Tickets); business rules in PRD §8.

## By item

Every item in `docs/progress/features.json` lists its `refs` (TD§, INV-, ARCH§, AD-, FR-, PRD§). Read those sections, plus the "Always" list above: `go run scripts/brief/main.go -item <ID>`.
