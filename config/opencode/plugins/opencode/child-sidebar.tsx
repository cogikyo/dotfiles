/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from '@opencode-ai/plugin/tui'
import { Show, createMemo, createSignal, type Accessor } from 'solid-js'

const id = 'opencode-child-sidebar'
const SIDE_KEY = 'sidebar_side'
const SAVED_KEY = 'sidebar_side_saved'
const SIDEBAR_WIDTH = 42
const PATCHED_ROW = Symbol.for('cullyn.sidebar.row')

type Side = 'left' | 'right'

type Node = {
  width: number
  isDestroyed?: boolean
  flexDirection?: string
  add?: (child: Node, index?: number) => unknown
  remove?: (child: Node) => unknown
  getChildren?: () => Node[]
  [PATCHED_ROW]?: true
}

function readSide(api: TuiPluginApi): Side {
  return api.kv.get<Side>(SIDE_KEY, 'right') === 'left' ? 'left' : 'right'
}

function restoreBuiltIn(api: TuiPluginApi) {
  const saved = api.kv.get(SAVED_KEY)
  if (saved == null) return
  if (api.kv.get('sidebar') === 'hide') api.kv.set('sidebar', saved)
  api.kv.set(SAVED_KEY, undefined)
}

function childrenOf(node: Node): Node[] {
  try {
    return typeof node.getChildren === 'function' ? node.getChildren() : []
  } catch {
    return []
  }
}

function walk(node: Node, visit: (node: Node, kids: Node[]) => void) {
  const kids = childrenOf(node)
  visit(node, kids)
  for (const child of kids) walk(child, visit)
}

function isSidebar(node: Node) {
  return Math.round(node.width) === SIDEBAR_WIDTH
}

function patchRow(node: Node, getSide: () => Side) {
  if (node[PATCHED_ROW]) return
  const proto = Object.getPrototypeOf(node) as object | null
  const desc = proto ? Object.getOwnPropertyDescriptor(proto, 'flexDirection') : undefined
  if (!desc?.set) return

  Object.defineProperty(node, 'flexDirection', {
    configurable: true,
    set(value: unknown) {
      desc.set!.call(node, getSide() === 'left' && value === 'row' ? 'row-reverse' : value)
    },
  })
  node[PATCHED_ROW] = true
}

function placeSidebar(row: Node, sidebar: Node, side: Side) {
  if (typeof row.remove !== 'function' || typeof row.add !== 'function') return
  const kids = childrenOf(row)
  const index = kids.indexOf(sidebar)
  if (index < 0) return
  const want = side === 'left' ? 0 : kids.length - 1
  if (index === want) return
  row.remove(sidebar)
  row.add(sidebar, want)
}

function applySide(root: Node | undefined, side: Side, getSide: () => Side) {
  if (!root || root.isDestroyed) return 0
  let found = 0

  walk(root, (node, kids) => {
    if (node.isDestroyed || kids.length !== 2) return
    const sidebar = kids.find(isSidebar)
    if (!sidebar) return

    found += 1
    patchRow(node, getSide)
    node.flexDirection = 'row'
    placeSidebar(node, sidebar, side)
  })

  return found
}

function stolenBindings(api: TuiPluginApi) {
  return api.tuiConfig.keybinds.get('agent.list').map((binding) => ({
    ...binding,
    cmd: 'sidebar.side.toggle',
    desc: 'Toggle sidebar side',
  }))
}

function HostedSidebar(props: { api: TuiPluginApi; sessionID: string; side: Side }) {
  return (
    <box
      backgroundColor={props.api.theme.current.backgroundPanel}
      width={SIDEBAR_WIDTH}
      position="absolute"
      top={0}
      left={props.side === 'left' ? 0 : undefined}
      right={props.side === 'right' ? 0 : undefined}
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
          {props.api.ui.Slot({ name: 'sidebar_content', session_id: props.sessionID })}
        </box>
      </scrollbox>
    </box>
  )
}

function ChildSidebar(props: { api: TuiPluginApi; side: Accessor<Side> }) {
  const sessionID = createMemo(() => {
    const route = props.api.route.current
    if (route.name !== 'session') return undefined

    const sessionID = route.params?.sessionID
    if (typeof sessionID !== 'string') return undefined
    return props.api.state.session.get(sessionID)?.parentID ? sessionID : undefined
  })

  return (
    <Show when={sessionID()} keyed>
      {(current) => <HostedSidebar api={props.api} sessionID={current} side={props.side()} />}
    </Show>
  )
}

const tui: TuiPlugin = async (api) => {
  restoreBuiltIn(api)

  const [side, setSide] = createSignal<Side>(readSide(api))
  const root = () => api.renderer.root as unknown as Node | undefined
  const apply = () => {
    try {
      return applySide(root(), side(), side)
    } catch (error) {
      console.error('sidebar side apply failed', error)
      return 0
    }
  }

  const toggle = () => {
    const next = side() === 'left' ? 'right' : 'left'
    api.kv.set(SIDE_KEY, next)
    setSide(next)
    const found = apply()
    api.renderer.requestRender()
    api.ui.dialog.clear()
    api.ui.toast({
      message: found ? `Sidebar ${next}` : `Sidebar ${next}, layout not found`,
    })
  }

  api.keymap.registerLayer({
    priority: 10_000,
    commands: [
      {
        name: 'sidebar.side.toggle',
        title: 'Toggle sidebar side',
        category: 'Session',
        namespace: 'palette',
        slashName: 'sidebar',
        run: toggle,
      },
    ],
    bindings: [
      ...stolenBindings(api),
      {
        key: '<leader>a',
        cmd: 'sidebar.side.toggle',
        desc: 'Toggle sidebar side',
      },
    ],
  })

  apply()
  const timer = setInterval(apply, 250)
  api.lifecycle.onDispose(() => clearInterval(timer))

  api.slots.register({
    order: 100,
    slots: {
      app() {
        return <ChildSidebar api={api} side={side} />
      },
    },
  })
}

const plugin: TuiPluginModule & { id: string } = {
  id,
  tui,
}

export default plugin
