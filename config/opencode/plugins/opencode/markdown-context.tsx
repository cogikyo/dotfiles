/** @jsxImportSource @opentui/solid */
import type { Message, ToolPart } from '@opencode-ai/sdk/v2'
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from '@opencode-ai/plugin/tui'
import { createTextAttributes } from '@opentui/core'
import path from 'node:path'
import { For, Show, createSignal, onCleanup } from 'solid-js'
import { colors } from '../shared/colors.ts'

const id = 'opencode-markdown-context'
const MAX_READS = 8
const MAX_LABEL_LENGTH = 30
const BOLD = createTextAttributes({ bold: true })

type MarkdownContextItem = {
  key: string
  path: string
  label: string
  compacted: boolean
  time: number
}

function MarkdownContext(props: { api: TuiPluginApi; sessionID: string }) {
  const [revision, setRevision] = createSignal(0)
  const [expanded, setExpanded] = createSignal(true)
  const refresh = () => setRevision((value) => value + 1)

  const disposers = [
    props.api.event.on('message.updated', (event) => {
      if (event.properties.sessionID === props.sessionID) refresh()
    }),
    props.api.event.on('message.removed', (event) => {
      if (event.properties.sessionID === props.sessionID) refresh()
    }),
    props.api.event.on('message.part.updated', (event) => {
      if (event.properties.sessionID === props.sessionID) refresh()
    }),
    props.api.event.on('message.part.removed', (event) => {
      if (event.properties.sessionID === props.sessionID) refresh()
    }),
    props.api.event.on('session.compacted', (event) => {
      if (event.properties.sessionID === props.sessionID) refresh()
    }),
  ]
  onCleanup(() => {
    for (const dispose of disposers) dispose()
  })

  const items = () => {
    revision()
    return markdownContextItems(props.api, props.sessionID)
  }

  return (
    <Show when={items().length > 0}>
      <box flexDirection="column" gap={0}>
        <box flexDirection="row" gap={0} onMouseDown={() => setExpanded((value) => !value)}>
          <text fg={props.api.theme.current.text} wrapMode="none">
            {expanded() ? '▼ ' : '▶ '}
          </text>
          <text fg={props.api.theme.current.text} attributes={BOLD}>Markdown Context</text>
          <text fg={props.api.theme.current.textMuted}>{` ${items().length} read`}</text>
        </box>
        <Show when={expanded()}>
          <For each={items()}>
            {(item) => (
              <box flexDirection="row" gap={0}>
                <text fg={sourceColor(props.api, item)} wrapMode="none">
                  {item.compacted ? 'C ' : 'R '}
                </text>
                <text fg={props.api.theme.current.textMuted} wrapMode="none">
                  {item.label}
                </text>
              </box>
            )}
          </For>
        </Show>
      </box>
    </Show>
  )
}

function markdownContextItems(api: TuiPluginApi, sessionID: string) {
  const reads = new Map<string, MarkdownContextItem>()
  const messages = api.state.session.messages(sessionID) as ReadonlyArray<Message>

  for (const message of messages) {
    for (const part of api.state.part(message.id)) {
      const item = markdownReadItem(api, part)
      if (!item) continue

      const existing = reads.get(item.key)
      if (!existing || item.time >= existing.time) reads.set(item.key, item)
    }
  }

  return Array.from(reads.values()).sort((left, right) => right.time - left.time).slice(0, MAX_READS)
}

function markdownReadItem(api: TuiPluginApi, part: ReturnType<TuiPluginApi['state']['part']>[number]): MarkdownContextItem | undefined {
  if (part.type !== 'tool' || !isReadTool(part.tool)) return undefined
  const tool = part as ToolPart
  if (tool.state.status !== 'completed') return undefined

  const filePath = markdownPathFromInput(tool.state.input)
  if (!filePath) return undefined

  return {
    key: filePath,
    path: filePath,
    label: displayPath(api, filePath),
    compacted: tool.state.time.compacted !== undefined,
    time: tool.state.time.end,
  }
}

function markdownPathFromInput(input: Record<string, unknown>) {
  for (const key of ['filePath', 'path', 'filepath', 'file']) {
    const value = input[key]
    if (typeof value === 'string' && isMarkdownPath(value)) return normalizeFilePath(value)
  }

  for (const value of Object.values(input)) {
    if (typeof value === 'string' && isMarkdownPath(value)) return normalizeFilePath(value)
  }

  return undefined
}

function isReadTool(tool: string) {
  return tool === 'read' || tool === 'Read' || tool === 'file.read' || tool === 'file_read'
}

function isMarkdownPath(value: string) {
  return /\.(md|mdx|markdown)$/i.test(value.split(/[?#]/, 1)[0])
}

function normalizeFilePath(value: string) {
  const clean = value.replace(/^file:\/\//, '').split(/[?#]/, 1)[0]
  if (clean.startsWith('~/')) return path.join(process.env.HOME || '~', clean.slice(2))
  return clean
}

function displayPath(api: TuiPluginApi, filePath: string) {
  const cwd = api.state.path.directory || api.state.path.worktree || ''
  const home = process.env.HOME || ''
  let label = filePath

  if (cwd && filePath.startsWith(cwd + path.sep)) label = filePath.slice(cwd.length + 1)
  else if (home && filePath.startsWith(home + path.sep)) label = filePath.slice(home.length + 1)

  return compactPath(label)
}

function compactPath(label: string) {
  if (label.length <= MAX_LABEL_LENGTH) return label

  const parts = label.split(path.sep).filter(Boolean)
  if (parts.length <= 2) return truncateLabel(label)

  const leaf = parts.at(-1) ?? label
  const parent = parts.at(-2) ?? ''
  const root = parts[0]
  const candidates = [
    `${root}/.../${parent}/${leaf}`,
    `${root}/.../${leaf}`,
    `.../${parent}/${leaf}`,
    `.../${leaf}`,
  ]

  return candidates.find((candidate) => candidate.length <= MAX_LABEL_LENGTH) ?? truncateLabel(candidates.at(-1) ?? label)
}

function truncateLabel(label: string) {
  if (label.length <= MAX_LABEL_LENGTH) return label
  return `${label.slice(0, Math.max(0, MAX_LABEL_LENGTH - 3))}...`
}

function sourceColor(api: TuiPluginApi, item: MarkdownContextItem) {
  const c = colors(api.theme.current)
  if (item.compacted) return c.orange
  return c.green
}

const tui: TuiPlugin = async (api) => {
  api.slots.register({
    order: 120,
    slots: {
      sidebar_content(_ctx, props: { session_id: string }) {
        return <MarkdownContext api={api} sessionID={props.session_id} />
      },
    },
  })
}

const plugin: TuiPluginModule & { id: string } = {
  id,
  tui,
}

export default plugin
