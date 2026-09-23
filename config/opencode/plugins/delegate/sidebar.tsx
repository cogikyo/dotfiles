/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from '@opencode-ai/plugin/tui'
import type { Message, Session } from '@opencode-ai/sdk/v2'
import { For, Show, createMemo, createSignal } from 'solid-js'
import { COMPACTION_LIMIT, formatTokens } from '../shared/session.ts'
import { SidebarSection } from '../shared/sidebar-section.tsx'

const id = 'delegate-lanes'

type Usage = { model: string; tokens: number }

function delegate(session: Session) {
  const value = session.metadata?.delegate
  return value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined
}

function lane(session: Session) {
  const name = delegate(session)?.lane
  return typeof name === 'string' && name ? name : undefined
}

function tokens(message: Message) {
  if (message.role !== 'assistant') return 0
  const value = message.tokens
  return value.total || value.input + value.output + value.cache.read + value.cache.write
}

function usage(message: Message): Usage | undefined {
  if (message.role !== 'assistant') return undefined
  const measured = tokens(message)
  if (!measured) return undefined
  return { model: `${message.providerID}/${message.modelID}`, tokens: measured }
}

function createLanes(api: TuiPluginApi) {
  const [sessions, setSessions] = createSignal<Record<string, Session>>({})
  const [usages, setUsages] = createSignal<Record<string, Usage>>({})
  const [statuses, setStatuses] = createSignal<Record<string, string>>({})
  const loaded = new Set<string>()

  const put = (session: Session) => setSessions((current) => ({ ...current, [session.id]: session }))
  const measure = (sessionID: string, value: Usage) => setUsages((current) => ({ ...current, [sessionID]: value }))

  async function loadUsage(sessionID: string) {
    const result = await api.client.session.messages({ sessionID, limit: 5 })
    const latest = (result.data ?? []).map((item) => usage(item.info)).findLast(Boolean)
    if (latest) measure(sessionID, latest)
  }

  async function load(parentID: string) {
    if (loaded.has(parentID)) return
    loaded.add(parentID)
    const response = await api.client.session.children({ sessionID: parentID })
    if (response.error || !response.data) {
      loaded.delete(parentID)
      return
    }
    const children = response.data.filter((child) => lane(child))
    setSessions((current) => ({ ...current, ...Object.fromEntries(children.map((child) => [child.id, child])) }))
    await Promise.all(children.map((child) => loadUsage(child.id)))
  }

  const disposers = [
    api.event.on('session.created', (event) => {
      if (lane(event.properties.info)) put(event.properties.info)
    }),
    api.event.on('session.updated', (event) => {
      if (lane(event.properties.info)) put(event.properties.info)
    }),
    api.event.on('session.deleted', (event) => {
      const sessionID = event.properties.sessionID
      if (!sessions()[sessionID]) return
      setSessions(({ [sessionID]: _, ...rest }) => rest)
    }),
    api.event.on('message.updated', (event) => {
      if (!sessions()[event.properties.sessionID]) return
      const value = usage(event.properties.info)
      if (value) measure(event.properties.sessionID, value)
    }),
    api.event.on('session.status', (event) => {
      if (!sessions()[event.properties.sessionID]) return
      setStatuses((current) => ({ ...current, [event.properties.sessionID]: event.properties.status.type }))
    }),
  ]
  api.lifecycle.onDispose(() => disposers.forEach((dispose) => dispose()))

  function forParent(parentID: string) {
    const newest = new Map<string, Session>()
    for (const child of Object.values(sessions())) {
      const name = lane(child)
      if (!name || child.parentID !== parentID) continue
      const seen = newest.get(name)
      if (!seen || child.time.created > seen.time.created) newest.set(name, child)
    }
    return newest
  }

  function status(session: Session) {
    if (delegate(session)?.context) return 'context-limited'
    return statuses()[session.id] ?? api.state.session.status(session.id)?.type ?? 'idle'
  }

  return { load, forParent, status, usage: (sessionID: string) => usages()[sessionID] }
}

type Lanes = ReturnType<typeof createLanes>

function Panel(props: { api: TuiPluginApi; lanes: Lanes; sessionID: string }) {
  void props.lanes.load(props.sessionID)
  const active = createMemo(() => props.lanes.forParent(props.sessionID))
  const names = createMemo(() => [...active().keys()].sort())

  return (
    <Show when={names().length}>
      <SidebarSection api={props.api} title="Lanes" detail={names().length}>
        <box flexDirection="column" paddingLeft={2}>
          <For each={names()}>{(name) => {
            const child = () => active().get(name)!
            const model = () => {
              const measured = props.lanes.usage(child().id)
              if (measured) return measured.model
              const pinned = child().model
              return pinned ? `${pinned.providerID}/${pinned.id}` : 'unknown model'
            }
            const spent = () => formatTokens(props.lanes.usage(child().id)?.tokens ?? 0)
            return (
              <box flexDirection="column">
                <text fg={props.api.theme.current.text}>{name}</text>
                <text fg={props.api.theme.current.textMuted}>{child().agent ?? 'unknown'} · {model()}</text>
                <text fg={props.api.theme.current.textMuted}>{props.lanes.status(child())} · {spent()}/{formatTokens(COMPACTION_LIMIT)}</text>
              </box>
            )
          }}</For>
        </box>
      </SidebarSection>
    </Show>
  )
}

const tui: TuiPlugin = async (api) => {
  const lanes = createLanes(api)
  api.slots.register({
    order: 200,
    slots: {
      sidebar_content(_ctx, props: { session_id: string }) {
        return (
          <Show when={!api.state.session.get(props.session_id)?.parentID}>
            <Panel api={api} lanes={lanes} sessionID={props.session_id} />
          </Show>
        )
      },
    },
  })
}

export default { id, tui } satisfies TuiPluginModule & { id: string }
