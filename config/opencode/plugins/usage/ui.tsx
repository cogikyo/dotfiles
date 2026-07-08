/** @jsxImportSource @opentui/solid */
import type { TuiPluginApi } from "@opencode-ai/plugin/tui";
import { createTextAttributes, type RGBA } from "@opentui/core";
import { For, Show } from "solid-js";
import { usageColor } from "../shared/colors.ts";
import type { ProviderUsage } from "./types.ts";
import type { TuiThemeCurrent } from "@opencode-ai/plugin/tui";

// Note color follows noteKind: info muted, warn amber, error red.
// Undefined noteKind is the legacy hard-error path (e.g. recordError "429") and stays red.
// Stale age overlays on live windows are the one exception that stays muted.
function noteColor(theme: TuiThemeCurrent, provider: ProviderUsage) {
  if (provider.noteKind === "info") return theme.textMuted;
  if (provider.noteKind === "warn") return theme.warning;
  if (provider.noteKind === "error") return theme.error;
  if (provider.windows.length > 0 && provider.note?.startsWith("stale ")) {
    return theme.textMuted;
  }
  return theme.error;
}

const BAR_WIDTH = 10;
const DURATION_WIDTH = 7;
const EXACT_WIDTH = 10;
const PERCENT_WIDTH = 3;
const DASH = "--";
const PLACEHOLDER_LABELS = ["H", "W"];
const BOLD = createTextAttributes({ bold: true });

type ResetParts = {
  duration: string;
  exact: string;
};

function formatReset(resetAt?: string): ResetParts {
  if (!resetAt) return { duration: "--", exact: "--" };
  const resetDate = new Date(resetAt);
  const ms = resetDate.getTime() - Date.now();
  if (!Number.isFinite(ms)) return { duration: "--", exact: "--" };
  if (ms <= 0) return { duration: "now", exact: "now" };

  const totalMinutes = Math.ceil(ms / 60_000);
  const days = Math.floor(totalMinutes / (24 * 60));
  const hours = Math.floor((totalMinutes % (24 * 60)) / 60);
  const minutes = totalMinutes % 60;

  const duration =
    days > 0
      ? `${days}d ${String(hours).padStart(2, "0")}h`
      : hours > 0
        ? `${hours}h ${String(minutes).padStart(2, "0")}m`
        : `${String(minutes).padStart(2, "0")}m`;

  const now = new Date();
  const sameDay = resetDate.toDateString() === now.toDateString();
  const time = new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(resetDate);
  const date = `${resetDate.getMonth() + 1}/${String(resetDate.getDate()).padStart(2, "0")}`;

  return {
    duration,
    exact: sameDay ? time : `${date} ${time}`,
  };
}

function usageBar(percent: number) {
  const filled = Math.max(
    0,
    Math.min(BAR_WIDTH, Math.round((percent / 100) * BAR_WIDTH)),
  );
  return "█".repeat(filled) + "░".repeat(BAR_WIDTH - filled);
}

function formatPercent(percent: number) {
  const rounded = Math.max(0, Math.min(100, Math.round(percent)));
  return rounded >= 100 ? "100" : `${String(rounded).padStart(2, "0")}%`;
}

// One usage row with fixed column widths. Real windows pass colored percent/bar; placeholder
// rows for windowless providers pass muted dashes and an empty bar so alignment never shifts.
function WindowRow(props: {
  theme: TuiThemeCurrent;
  label: string;
  percent: string;
  percentColor: RGBA;
  bar: string;
  barColor: RGBA;
  duration: string;
  exact: string;
}) {
  return (
    <box flexDirection="row" gap={0}>
      <text fg={props.theme.textMuted}>{props.label.padEnd(2, " ")}</text>
      <text fg={props.percentColor}>{`${props.percent} `}</text>
      <text fg={props.barColor}>{`${props.bar} `}</text>
      <text fg={props.theme.textMuted}>
        {`${props.duration.padStart(DURATION_WIDTH, " ")} `}
      </text>
      <box flexGrow={1} />
      <text fg={props.theme.textMuted}>
        {props.exact.padStart(EXACT_WIDTH, " ")}
      </text>
    </box>
  );
}

export function UsageDashboard(props: {
  api: TuiPluginApi;
  providers: ProviderUsage[];
  activeProviderID: string;
  refreshingProviderIDs?: Set<string>;
  onRefresh?: (providerID: string) => void;
}) {
  const theme = () => props.api.theme.current;
  return (
    <box flexDirection="column" gap={0} paddingLeft={1}>
      <For each={props.providers}>
        {(provider) => {
          const refreshing = () =>
            props.refreshingProviderIDs?.has(provider.id) ?? false;
          // In-flight manual refresh: primary label, and "refreshing" only when no real note
          // so 429/error/stale text stays visible once the fetch returns (or was already there).
          const labelColor = () =>
            refreshing() || provider.id === props.activeProviderID
              ? theme().primary
              : theme().text;
          return (
            <box flexDirection="column" gap={0}>
              <box
                flexDirection="row"
                gap={0}
                onMouseDown={() => props.onRefresh?.(provider.id)}
              >
                <text fg={labelColor()} attributes={BOLD}>
                  {provider.label}
                </text>
                <Show when={provider.note}>
                  <text fg={noteColor(theme(), provider)}>
                    {` ${provider.note}`}
                  </text>
                </Show>
                <Show when={refreshing() && !provider.note}>
                  <text fg={theme().primary}>{` refreshing`}</text>
                </Show>
              </box>
              <Show
                when={provider.windows.length > 0}
                fallback={
                  <For each={provider.placeholders ?? PLACEHOLDER_LABELS}>
                    {(label) => (
                      <WindowRow
                        theme={theme()}
                        label={label}
                        percent={DASH.padEnd(PERCENT_WIDTH, " ")}
                        percentColor={theme().textMuted}
                        bar={"░".repeat(BAR_WIDTH)}
                        barColor={theme().textMuted}
                        duration={DASH}
                        exact={DASH}
                      />
                    )}
                  </For>
                }
              >
                <For each={provider.windows}>
                  {(window) => {
                    const reset = formatReset(window.resetAt);
                    const pct = window.usedPercent;
                    // Unknown percent (e.g. xAI weekly): muted "--" cell and empty bar, but keep
                    // the real duration/exact reset columns so alignment matches healthy rows.
                    return (
                      <WindowRow
                        theme={theme()}
                        label={window.label}
                        percent={
                          pct !== undefined
                            ? formatPercent(pct)
                            : DASH.padEnd(PERCENT_WIDTH, " ")
                        }
                        percentColor={
                          pct !== undefined
                            ? usageColor(theme(), pct)
                            : theme().textMuted
                        }
                        bar={
                          pct !== undefined
                            ? usageBar(pct)
                            : "░".repeat(BAR_WIDTH)
                        }
                        barColor={
                          pct !== undefined
                            ? usageColor(theme(), pct)
                            : theme().textMuted
                        }
                        duration={reset.duration}
                        exact={reset.exact}
                      />
                    );
                  }}
                </For>
              </Show>
            </box>
          );
        }}
      </For>
    </box>
  );
}
