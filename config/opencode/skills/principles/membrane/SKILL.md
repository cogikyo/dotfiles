---
name: membrane
description: "Use when wiring system boundaries, validation, framework adapters, or side effects."
---

# Membrane

Translate and validate at the edge so the core can trust its types.
A boundary turns outside claims into domain data so internal code can work from one established contract.

- Make IO, DTO, transport, storage, and framework shapes explicit at boundaries; parse outside claims into validated domain types.
- Check API/HTTP/RPC, database, UI, shell/process/filesystem, config/env/secrets, and LLM/prompt/agent crossings.
- Contain effects, logging, formatting, retries, and auth in the shell that owns them; keep domain decisions pure.
- Validate a crossing or unestablished invariant once; reject repeated checks of established domain facts.
- Keep frontend, backend, and model names aligned for the same concept.
- Avoid extracting a pass-through layer just to label it a boundary.

See: `legibility`, `etiology`.
