/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginApi, TuiPluginModule, TuiSidebarMcpItem } from "@opencode-ai/plugin/tui";
import { For, Show, createMemo } from "solid-js";
import { SidebarSection } from "../shared/sidebar-section.tsx";

const id = "opencode-mcp";

/** TUI plugin that lists MCP server status and failures in the sidebar when any server is enabled. */
const tui: TuiPlugin = async (api) => {
  api.slots.register({
    order: 190,
    slots: {
      sidebar_content() {
        return <Mcp api={api} />;
      },
    },
  });
};

export default { id, tui } satisfies TuiPluginModule & { id: string };

const LABELS: Record<TuiSidebarMcpItem["status"], string> = {
  connected: "Connected",
  disabled: "Disabled",
  failed: "Failed",
  needs_auth: "Needs auth",
  needs_client_registration: "Needs client ID",
};

function tone(api: TuiPluginApi, status: TuiSidebarMcpItem["status"]) {
  const theme = api.theme.current;
  if (status === "connected") return theme.success;
  if (status === "needs_auth") return theme.warning;
  if (status === "disabled") return theme.textMuted;
  return theme.error;
}

function Mcp(props: { api: TuiPluginApi }) {
  const servers = createMemo(() => props.api.state.mcp());
  const enabled = createMemo(() => servers().some((server) => server.status !== "disabled"));
  const detail = createMemo(() => {
    const active = servers().filter((server) => server.status === "connected").length;
    const errors = servers().filter((server) => server.status !== "connected" && server.status !== "disabled").length;
    return errors ? `${active} active, ${errors} error${errors > 1 ? "s" : ""}` : `${active} active`;
  });

  return (
    <Show when={enabled()}>
      <SidebarSection api={props.api} title="MCP" detail={detail()}>
        <box flexDirection="column">
          <For each={servers()}>
            {(server) => (
              <box flexDirection="row" gap={1}>
                <text fg={tone(props.api, server.status)} flexShrink={0}>
                  •
                </text>
                <text fg={props.api.theme.current.text} wrapMode="none">
                  {server.name}
                  <span style={{ fg: props.api.theme.current.textMuted }}>
                    {` ${server.status === "failed" && server.error ? server.error : LABELS[server.status]}`}
                  </span>
                </text>
              </box>
            )}
          </For>
        </box>
      </SidebarSection>
    </Show>
  );
}
