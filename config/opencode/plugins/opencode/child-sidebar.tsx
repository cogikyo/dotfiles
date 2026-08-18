/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from '@opencode-ai/plugin/tui'
import { Show, createMemo } from 'solid-js'

const id = 'opencode-child-sidebar'

function ChildSidebar(props: { api: TuiPluginApi }) {
  const sessionID = createMemo(() => {
    const route = props.api.route.current
    if (route.name !== 'session') return undefined

    const sessionID = route.params?.sessionID
    if (typeof sessionID !== 'string') return undefined
    return props.api.state.session.get(sessionID)?.parentID ? sessionID : undefined
  })

  return (
    <Show when={sessionID()} keyed>
      {(current) => (
        <box
          backgroundColor={props.api.theme.current.backgroundPanel}
          width={42}
          position="absolute"
          top={0}
          right={0}
          bottom={4}
          paddingTop={1}
          paddingBottom={1}
          paddingLeft={2}
          paddingRight={2}
        >
          <scrollbox
            flexGrow={1}
            verticalScrollbarOptions={{
              trackOptions: {
                backgroundColor: props.api.theme.current.background,
                foregroundColor: props.api.theme.current.borderActive,
              },
            }}
          >
            <box flexShrink={0} gap={1} paddingRight={1}>
              {props.api.ui.Slot({ name: 'sidebar_content', session_id: current })}
            </box>
          </scrollbox>
        </box>
      )}
    </Show>
  )
}

const tui: TuiPlugin = async (api) => {
  api.slots.register({
    order: 100,
    slots: {
      app() {
        return <ChildSidebar api={api} />
      },
    },
  })
}

const plugin: TuiPluginModule & { id: string } = {
  id,
  tui,
}

export default plugin
