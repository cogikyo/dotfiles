import type { Plugin } from "@opencode/plugin/tui";
import { createMemo, createSignal, For, Show } from "solid-js";
import { clip } from "../byline/notice.ts";
import { glyphs } from "../lanes/label.ts";
import { hover, spinner } from "../context/section.tsx";
import type { Live } from "./live.ts";
import { roles, topic, type Step } from "./parse.ts";
import { complete, symbols, type Phase } from "./progress.ts";
import { icon, ink, palette, shades, type Row, type Segment, type Tone } from "./style.ts";

type Context = Plugin.Context;

const quiet: Partial<Record<Phase, Tone>> = { done: "muted", skipped: "muted", failed: "error" };

export function sidebar(context: Context, live: Live, open: (sessionID: string) => void) {
  context.ui.slot({
    append: "sidebar.content",
    render: (input) => (
      <Sidebar context={context} live={live} sessionID={input.sessionID} open={() => open(input.sessionID)} />
    ),
  });
}

function Sidebar(props: { context: Context; live: Live; sessionID: string; open: () => void }) {
  const [folded, setFolded] = createSignal(false);
  const [room, setRoom] = createSignal(0);
  const shown = createMemo(() => props.live.shown(props.sessionID));
  const tones = createMemo(() => palette(props.context.theme, props.context.themeMode));
  const frame = spinner(() => Object.values(shown()?.progress.phases ?? {}).includes("running"));
  const theme = () => props.context.theme.text;
  const digits = () => Math.max(1, ...(shown()?.workflow.steps.map(({ n }) => String(n).length) ?? []));

  return (
    <Show when={shown()}>
      {(current) => {
        const steps = () => current().workflow.steps;
        const states = () => current().progress.phases;
        const done = () => steps().flatMap(({ n }) => (states()[n] === "done" ? [n] : []));
        const paused = () => current().progress.status === "paused";
        const count = () => ` ${complete(states())}/${steps().length}`;
        const title = () => topic(current().workflow.title) || "Workflow";
        const lines = () => fold(title(), room(), Bun.stringWidth(count()) + (paused() ? 7 : 0));
        return (
          <box flexDirection="column">
            <box flexDirection="row">
              <text fg={theme().base} wrapMode="none" flexShrink={0} onMouseDown={() => setFolded((value) => !value)}>
                {folded() ? "▶ " : "▼ "}
              </text>
              <box
                flexDirection="column"
                flexGrow={1}
                minWidth={0}
                onMouseDown={() => props.open()}
                onSizeChange={function () {
                  setRoom(this.width);
                }}
              >
                <text fg={theme().base} wrapMode="none">
                  <b>{lines()[0]}</b>
                  <span style={{ fg: theme().muted }}>{count()}</span>
                  <Show when={paused()}>
                    <span style={{ fg: tones().yellow }}> paused</span>
                  </Show>
                </text>
                <For each={lines().slice(1)}>
                  {(text) => (
                    <text fg={theme().base} wrapMode="none">
                      <b>{text}</b>
                    </text>
                  )}
                </For>
              </box>
            </box>
            <Show when={!folded()}>
              <Show when={done().length}>
                <Done context={props.context} numbers={done()} open={props.open} />
              </Show>
              <For each={steps().filter((step) => states()[step.n] !== "done")}>
                {(step) => (
                  <Row
                    context={props.context}
                    step={step}
                    phase={states()[step.n]}
                    digits={digits()}
                    frame={frame()}
                    agent={current().agents[step.n]}
                    open={props.open}
                  />
                )}
              </For>
            </Show>
          </box>
        );
      }}
    </Show>
  );
}

function Row(props: {
  context: Context;
  step: Step;
  phase: Phase;
  digits: number;
  frame: string;
  agent?: string;
  open: () => void;
}) {
  const [hovered, setHovered] = createSignal(false);
  const [room, setRoom] = createSignal(0);
  const tones = createMemo(() => palette(props.context.theme, props.context.themeMode));
  const cells = createMemo(() =>
    line(props.step, props.phase, room() || 37, { digits: props.digits, frame: props.frame, agent: props.agent }),
  );

  return (
    <box
      flexDirection="row"
      onMouseDown={() => props.open()}
      onMouseOver={() => setHovered(true)}
      onMouseOut={() => setHovered(false)}
      onSizeChange={function () {
        setRoom(this.width);
      }}
    >
      <text wrapMode="none">
        <For each={cells()}>
          {(cell, index) => (
            <span style={{ fg: hovered() && index() === 1 ? hover : tones()[cell.tone] }}>{cell.text}</span>
          )}
        </For>
      </text>
    </box>
  );
}

function Done(props: { context: Context; numbers: readonly number[]; open: () => void }) {
  const [hovered, setHovered] = createSignal(false);
  const [room, setRoom] = createSignal(0);
  const tones = createMemo(() => palette(props.context.theme, props.context.themeMode));
  const mark = `${symbols.done} `;
  const lines = createMemo(() => fold(props.numbers.join(", "), (room() || 37) - Bun.stringWidth(mark), 0));

  return (
    <box
      flexDirection="column"
      onMouseDown={() => props.open()}
      onMouseOver={() => setHovered(true)}
      onMouseOut={() => setHovered(false)}
      onSizeChange={function () {
        setRoom(this.width);
      }}
    >
      <For each={lines()}>
        {(text, index) => (
          <text wrapMode="none">
            <span style={{ fg: tones()[shades.done] }}>{index() ? " ".repeat(Bun.stringWidth(mark)) : mark}</span>
            <span style={{ fg: hovered() ? hover : tones()[quiet.done ?? "text"] }}>{text}</span>
          </text>
        )}
      </For>
    </box>
  );
}

function fold(text: string, width: number, reserve: number) {
  if (width <= 0) return [text];
  const lines: string[] = [];
  let current = "";
  for (const word of text.split(/\s+/).filter(Boolean)) {
    const room = (lines.length ? width : width - reserve) - Bun.stringWidth(current);
    if (current && Bun.stringWidth(word) + 1 > room) {
      lines.push(current);
      current = word;
    } else current = current ? `${current} ${word}` : word;
  }
  return [...lines, current];
}

function line(step: Step, phase: Phase, width: number, look: { digits: number; frame: string; agent?: string }): Row {
  const running = phase === "running";
  const coming = phase === "ready" || phase === "waiting";
  const hue = (look.agent && role(look.agent)) || ink(step.owner).tone;
  const glyph = coming ? icon(step.owner) : running ? look.frame : symbols[phase];
  const lead = `${glyph} ${String(step.n).padStart(look.digits)} `;
  const tag = look.agent ? label(look.agent, running ? hue : undefined) : [];
  const used = Bun.stringWidth(lead) + tag.reduce((sum, { text }) => sum + Bun.stringWidth(text), 0);
  const name = clip(topic(step.headline), Math.max(1, width - used - (tag.length ? 1 : 0)));
  const gap = width - used - Bun.stringWidth(name);
  return [
    { text: lead, tone: running || coming ? hue : shades[phase] },
    { text: name, tone: running ? hue : (quiet[phase] ?? "text") },
    ...(tag.length ? [{ text: " ".repeat(Math.max(1, gap)), tone: "muted" } as const, ...tag] : []),
  ];
}

function label(agent: string, tone?: Tone): Segment[] {
  const [kind = "", ...rest] = agent.split("/");
  const { role: mark, scope } = glyphs(agent);
  return [
    { text: `${scope ?? mark ?? "?"} `, tone: tone ?? role(agent) ?? "muted" },
    { text: rest.join("/") || kind, tone: tone ?? "muted" },
  ];
}

function role(agent: string) {
  const [kind] = agent.split("/");
  return roles.find((name) => name === kind);
}
