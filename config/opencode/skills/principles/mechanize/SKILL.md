---
name: mechanize
description: "Use when repeating an instruction or starting non-trivial work that needs a repeatable procedure or proof."
---

# Mechanize

A lesson needed twice becomes a lint, check, or script.
An ignored rule can fail silently; enforcement makes the error visible.
A rerunnable tool also lets another reader check non-trivial work without repeating it by hand.

- On the second correction, replace enforceable instructions with a lint, permission, plugin, check, or script.
- Do one unit by hand, then write the smallest tool that does the work or proves it.
- Compare the tool's result with that first unit and make reruns safe.
- Prefer a deterministic pass over delegates doing the same steps by hand.
- Do not claim this principle without a file the reviewer can inspect and rerun.

Some lessons still need judgment.
Put those instructions at the decision and show a concrete failure, such as claiming the installed program changed because a build passed.

See: `falsify`, `converge`.
