/** @jsxImportSource @opentui/solid */
import type { TuiPluginApi } from "@opencode-ai/plugin/tui";
import { createTextAttributes } from "@opentui/core";
import { For, Show } from "solid-js";
import { usageColor } from "../shared/colors.ts";
import type { ProviderUsage } from "./types.ts";

const BAR_WIDTH = 10;
const DURATION_WIDTH = 7;
const EXACT_WIDTH = 10;
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

export function UsageDashboard(props: {
  api: TuiPluginApi;
  providers: ProviderUsage[];
  activeProviderID: string;
}) {
  return (
    <box flexDirection="column" gap={0} paddingLeft={1}>
      <text fg={props.api.theme.current.text} attributes={BOLD}>Usage Limits</text>
      <For each={props.providers}>
        {(provider) => (
          <box flexDirection="column" gap={0}>
            <box flexDirection="row" gap={0}>
              <text
                fg={
                  provider.id === props.activeProviderID
                    ? props.api.theme.current.primary
                    : props.api.theme.current.text
                }
                attributes={BOLD}
              >
                {provider.label}
              </text>
            </box>
            <For each={provider.windows}>
              {(window) => (
                <box flexDirection="row" gap={0}>
                  <text fg={props.api.theme.current.textMuted}>
                    {window.label.padEnd(2, " ")}
                  </text>
                  <text fg={usageColor(props.api.theme.current, window.usedPercent)}>
                    {`${String(Math.round(window.usedPercent)).padStart(2, "0")}% `}
                  </text>
                  <text fg={usageColor(props.api.theme.current, window.usedPercent)}>
                    {`${usageBar(window.usedPercent)} `}
                  </text>
                  {(() => {
                    const reset = formatReset(window.resetAt);
                    return (
                      <>
                        <text fg={props.api.theme.current.textMuted}>
                          {`${reset.duration.padStart(DURATION_WIDTH, " ")} `}
                        </text>
                        <box flexGrow={1} />
                        <text fg={props.api.theme.current.textMuted}>
                          {reset.exact.padStart(EXACT_WIDTH, " ")}
                        </text>
                      </>
                    );
                  })()}
                </box>
              )}
            </For>
            <Show when={provider.note}>
              <text
                fg={
                  provider.windows.length > 0
                    ? props.api.theme.current.textMuted
                    : props.api.theme.current.error
                }
              >
                {provider.note}
              </text>
            </Show>
          </box>
        )}
      </For>
    </box>
  );
}
