# 🌸 `Peony`

**A calm, CLI-first cognitive holding space for unfinished thoughts.**

> *Not a task manager, notes app or journal.*

---

## What is `Peony`?

`Peony` is a **local-first, humane CLI application** designed to help you capture, tend, and gently resolve half-formed thoughts—without forcing them into tasks, deadlines, or artificial structure.

Modern tools demand commitment too early:

* task managers ask *“when will you do this?”*
* notes apps become noisy dumping grounds
* journals require emotional energy on demand

**`Peony` offers a middle ground**:
a quiet place for thoughts that are *not ready yet*.

---

## Core Philosophy

### Thoughts are seeds, not obligations

A thought may:

* rest
* mature
* transform
* or dissolve entirely

`Peony` respects that process.

### Time-aware, not time-driven

`Peony` never nags.
Thoughts resurface **when they feel ready**, not when a reminder fires.

### Language matters

`Peony` speaks softly.
Commands are verbs, not flags.
There are no streaks, no scores, no guilt loops.

### Private by default

Everything is stored locally.
No accounts. No sync. No analytics.
Your inner life is not a dataset.

---

## What `Peony` Is *Not*

* ❌ Not a productivity dashboard
* ❌ Not a goal tracker
* ❌ Not an AI coach
* ❌ Not collaborative or social
* ❌ Not optimized for speed or scale

`Peony` is optimized for **clarity and care**.

---

## Core Concepts

### Cognitive Units (CUs)

The fundamental object in `Peony`.

A Cognitive Unit(CU) can represent:

* an unresolved decision
* a lingering worry
* an idea in early formation
* a memory fragment
* a question you’re not ready to answer

Each CU has:

* a lifecycle state
* a temporal context
* optional emotional metadata
* a gentle interaction history

---

## Lifecycle of a Thought

1. **Captured** – softly recorded, without classification pressure
2. **Resting** – intentionally left untouched
3. **Tended** – revisited when appropriate
4. **Evolved** – transformed into a task, note, or plan
5. **Released** – consciously let go
6. **Archived** – preserved without demand

Nothing ever “fails.”

---

## CLI-First Experience

`Peony` is designed to be **used from the terminal**, thoughtfully and slowly.

### Example interactions

```bash
`Peony` add
> What’s on your mind?
> "Unsure whether to double down on Go or consolidate Python first."
```

```bash
`Peony` tend
🌱 2 thoughts feel ready for reflection today.
```

```bash
`Peony` view
🌸 This thought has been resting for 14 days.
🌿 You last touched it late at night.
```

Commands are designed to feel **inviting**, not mechanical.

---

## CLI Commands

* `add` — capture a thought gently
* `tend` — surface thoughts ready for reflection
* `view` — read a thought in context
* `rest` — intentionally defer
* `evolve` — convert into a task / note (external)
* `release` — let go without guilt
* `archive` — long-term memory

### Planned for the frontend Eden integration, not CLI:
* `garden` — high-level overview

---

## Frontend: A Quiet Window

`Peony` includes an **optional, read-only frontend**—a window into your inner landscape.

### Purpose

* Visualize thought lifecycles
* Observe seasons of thinking
* Reflect without interaction pressure

### Design principles

* No metrics
* No dashboards
* No urgency signals
* Slow transitions
* Soft color palette

The frontend exists to **help you see**, not manage.

---

## Architecture Overview

```
`Peony`
├── Core Engine (Go)
│   ├── Thought lifecycle
│   ├── Temporal logic
│   └── Language system
│
├── CLI Interface
│   └── Bubble Tea + Lip Gloss
│
├── Storage
│   └── SQLite (local-first)
│
└── Read-only Frontend
    ├── GoTH + HTMX
    └── Tailwind (minimal, soft)
```

---

## Tech Stack

### Core

* **Go** — clarity, longevity, low cognitive overhead
* **SQLite** — durable, portable local storage

### CLI

* **Bubble Tea** — calm, state-driven terminal UX
* **Lip Gloss** — intentional visual hierarchy

### Frontend (optional)

* **GoTH + HTMX** — minimal, server-driven UI
* **Tailwind CSS** — restrained visual language

> No JavaScript framework is required for `Peony`’s core philosophy.

---

## Local-First by Design

* No cloud dependency
* No accounts
* No telemetry
* Your data lives with you

`Peony` will still work the same way in ten years.

---

## Intended Users

`Peony` is for people who:

* think deeply
* feel overwhelmed by premature structure
* value reflection over optimization
* prefer calm tools over clever ones

It is especially suited for:

* developers
* researchers
* writers
* designers
* long-horizon thinkers

---

## Project Status

`Peony` is **pre-frontend, CLI-complete**.
The focus is on:

* core lifecycle correctness
* language tone
* UX restraint

Feature creep is intentionally resisted.

---

## Roadmap (High Level)

* [x] v0.1 — Core CLI, lifecycle, local storage
* [x] v0.2 — 
* [x] v0.3 — Optional reflection aids (non-directive)
* [x] v0.4 — 
* [x] v0.5 — CLI polish, user feedback iteration
* [ ] v0.6 — Read-only frontend (Eden integration)
~
* [ ] v0.7 — Frontend polish and interactivity, user feedback iteration
* [ ] v1.0+ — Optional Semantic AI integration (non-prescriptive, reflective only)

AI integration, if ever added, will be:

* opt-in
* reflective only
* non-prescriptive

---

## Why “`Peony`”?

Peonies bloom slowly.
They do not rush, yet they are unmistakably full.

`Peony` exists for thoughts that need **time, space, and kindness**.

---

## License

MIT
You are free to use, modify, and learn from `Peony`—
just as gently as it was designed.

---

🌸
*Some thoughts don’t need solving.
They need somewhere safe to wait.*
