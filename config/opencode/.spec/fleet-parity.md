# Fleet parity

The entire agent fleet, provider set, and supporting configuration behave under V2 as they do under V1, proven by behavioral probes rather than schema acceptance, with configuration ported natively to V2 on the migration branch.

## Agents and skills

- All four primary modes and every leaf agent load under V2 with frontmatter semantics intact: mode, model pins, tool overrides, and permission overrides.
- The default agent choice and the disabled built-in agents carry over.
- Skills load and trigger under V2 as they do under V1.

## Providers and models

- The enabled provider set, custom provider definitions, model variants, and reasoning effort mappings resolve identically under V2.
- Default and small model choices carry over unchanged.

## Session settings

- Compaction, snapshot, watcher, and sharing behavior carry over where V2 honors the same concepts; where native V2 behavior satisfies the same invariant differently, native behavior wins and the V1-era setting is dropped.
- V1-era configuration keys with no V2 meaning are deleted from the migration branch rather than carried as inert residue.

## Acceptance

- Each primary mode completes one real task under V2 with its expected tools, permissions, and models.
