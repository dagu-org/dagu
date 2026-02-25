# RFC Guidelines

## Purpose

RFCs document **design decisions** — the *what* and *why*, not implementation details. No code, no file paths, no function signatures.

## Writing Principles

- **Concise** — Every sentence earns its place. Cut filler.
- **Precise** — Use exact terms.
- **Scannable** — Grasp in under 5 minutes.
- **Accurate** — Ground every claim in the current codebase and latest online documentation if needed. Zero speculation.
- **Visual** — Use mermaid diagrams for sequential diagram, state machines, and data relationships.

## Required Sections

- Goal: 1–3 sentences. What and why.
- Scope: In-scope vs out-of-scope table.
- Solution: Architectural design with mermaid diagrams, configuration (fields/types/defaults), API changes, and 1–2 examples.
- Data Model: New or modified stored state as a field table (name, type, default, description).
- Edge Cases & Tradeoffs: Decisions table: what was chosen, what was considered, why.
- Definition of Done: Testable assertions that confirm the RFC is fully implemented.

