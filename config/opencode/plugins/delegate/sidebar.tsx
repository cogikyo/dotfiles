/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from '@opencode-ai/plugin/tui'
import type { Message, Session } from '@opencode-ai/sdk/v2'
import { For, Show, createSignal, onCleanup } from 'solid-js'
import { COMPACTION_LIMIT, formatTokens } from '../shared/session.ts'
import { SidebarSection } from '../shared/sidebar-section.tsx'

const id = 'delegate-lanes'

function lane(session: Session) {
  const delegate = session.metadata?.delegate
  if (!delegate || typeof delegate !== 'object') return undefined
  const name = (delegate as Record<string, unknown>).lane
  return typeof name === 'string' && name ? name : undefined
}

function limited(session: Session) {
  const delegate = session.metadata?.delegate
  if (!delegate || typeof delegate !== 'object') return false
  return Boolean((delegate as Record<string, unknown>).context)
}

function latestAssistant(messages: ReadonlyArray<Message>) {
  return messages.findLast((message) => message.role === 'assistant')
}

function tokens(message: Message | undefined) {
  if (!message || message.role !== 'assistant') return 0
  const value = message.tokens
  return value.total || value.input + value.output + value.cache.read + value.cache.write
}

function Lanes(props: { api: TuiPluginApi; sessionID: string }) {
  const [children, setChildren] = createSignal<Session[]>([])
  const [messages, setMessages] = createSignal<Record<string, Message>>({})
  const [usage, setUsage] = createSignal<Record<string, number>>({})
  const [revision, setRevision] = createSignal(0)

  void props.api.client.session.children({ sessionID: props.sessionID }).then((response) => {
    if (response.error || !response.data) return
    setChildren(response.data)
    for (const child of response.data.filter((item) => lane(item))) {
      void props.api.client.session.messages({ sessionID: child.id }).then((result) => {
        const all = (result.data ?? []).map((item) => item.info)
        const message = latestAssistant(all)
        if (message) setMessages((current) => ({ ...current, [child.id]: message }))
        const measured = [...all].reverse().find((item) => tokens(item) > 0)
        if (measured) setUsage((current) => ({ ...current, [child.id]: tokens(measured) }))
      })
    }
  })

  const updated = props.api.event.on('session.updated', (event) => {
    const session = event.properties.info
    if (session.parentID !== props.sessionID || !lane(session)) return
    setChildren((current) => [...current.filter((item) => item.id !== session.id), session])
  })
  const deleted = props.api.event.on('session.deleted', (event) => {
    setChildren((current) => current.filter((item) => item.id !== event.properties.sessionID))
  })
  const messageUpdated = props.api.event.on('message.updated', (event) => {
    if (event.properties.info.role !== 'assistant') return
    setMessages((current) => ({ ...current, [event.properties.sessionID]: event.properties.info }))
    const measured = tokens(event.properties.info)
    if (measured) setUsage((current) => ({ ...current, [event.properties.sessionID]: measured }))
  })
  const statusUpdated = props.api.event.on('session.status', () => setRevision((value) => value + 1))
  onCleanup(updated)
  onCleanup(deleted)
  onCleanup(messageUpdated)
  onCleanup(statusUpdated)

  const active = () => {
    revision()
    const byName = new Map<string, Session>()
    for (const child of [...children()].sort((a, b) => b.time.created - a.time.created)) {
      const name = lane(child)
      if (name && !byName.has(name)) byName.set(name, child)
    }
    return [...byName.entries()]
  }

  return (
    <Show when={active().length}>
      <SidebarSection api={props.api} title="Lanes" detail={active().length}>
        <box flexDirection="column" paddingLeft={2}>
          <For each={active()}>{([name, child]) => {
            const message = () => messages()[child.id]
            const status = () => limited(child) ? 'context-limited' : props.api.state.session.status(child.id)?.type ?? 'idle'
            const model = () => {
              const info = message()
              if (info?.role === 'assistant') return `${info.providerID}/${info.modelID}`
              return child.model ? `${child.model.providerID}/${child.model.id}` : 'unknown model'
            }
            return (
              <box flexDirection="column">
                <text fg={props.api.theme.current.text}>{name}</text>
                <text fg={props.api.theme.current.textMuted}>{child.agent ?? 'unknown'} · {model()}</text>
                <text fg={props.api.theme.current.textMuted}>{status()} · {formatTokens(usage()[child.id] ?? 0)}/{formatTokens(COMPACTION_LIMIT)}</text>
              </box>
            )
          }}</For>
        </box>
      </SidebarSection>
    </Show>
  )
}

const tui: TuiPlugin = async (api) => {
  api.slots.register({
    order: 200,
    slots: {
      sidebar_content(_ctx, props: { session_id: string }) {
        if (api.state.session.get(props.session_id)?.parentID) return null
        return <Lanes api={api} sessionID={props.session_id} />
      },
    },
  })
}

export default { id, tui } satisfies TuiPluginModule & { id: string }
