import { items, parse, type Step } from "../workflow/parse.ts";
import { settled, waits, type Flow } from "../workflow/state.ts";

export type Verdict = { step: Step } | { error: string };

export function admit(state: { flow?: Flow } | undefined, n: number, linked: boolean): Verdict {
  if (!state)
    return {
      error: `The workflow plugin didn't answer, so step ${n} can't be checked or linked. Retry, or launch without \`step\`.`,
    };
  const { flow } = state;
  if (!flow)
    return {
      error: `No active workflow is stored, so step ${n} has nothing to link to. Propose a \`\`\`workflow fence and call \`workflow\` with \`start\`, or launch without \`step\`.`,
    };
  if (flow.status === "paused")
    return {
      error: `Workflow rev ${flow.rev} is paused for a replan. Finish the replan and call \`workflow\` with \`start\` before dispatching step ${n}.`,
    };
  const steps = parse(flow.source).steps;
  const step = steps.find((candidate) => candidate.n === n);
  if (!step)
    return { error: `Rev ${flow.rev} has no step ${n}; its steps are ${steps.map((known) => known.n).join(", ")}.` };
  if (linked) return { step };
  if (settled(flow, n)) {
    const mark = flow.marks[n];
    const gated = steps.some((gate) => gate.repair?.to.includes(n));
    return {
      error: gated
        ? `Step ${n} is marked ${mark}. Call \`workflow\` with \`repair: [${n}]\` first to reopen it and spend a round of its gate's budget.`
        : `Step ${n} is marked ${mark} and no gate repairs it. Revise the step and call \`workflow\` with \`start\` if it needs another pass.`,
    };
  }
  const unmet = waits(flow, step);
  if (unmet.length) {
    const many = unmet.length > 1;
    return {
      error: `Step ${n} waits on step${many ? "s" : ""} ${unmet.join(", ")}. Mark ${many ? "them" : "it"} done with \`workflow\` once ${many ? "their results are" : "its result is"} accepted.`,
    };
  }
  return { step };
}

export function checklist(step: Step) {
  const list = items(step);
  const done = step.details.find(({ kind }) => kind === "done");
  if (!list.length && !done) return undefined;
  return [
    `Workflow step ${step.n}: ${step.headline}`,
    ...list.map(({ n, text }) => `${n}. ${text}`),
    ...(done ? [`done: ${done.text}`] : []),
    ...(list.length ? ["Tick items with `workflow({ check: [n…] })` as they complete."] : []),
  ].join("\n");
}
