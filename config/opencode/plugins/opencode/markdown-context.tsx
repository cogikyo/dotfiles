/** @jsxImportSource @opentui/solid */
import type { Message, ToolPart } from '@opencode-ai/sdk/v2'
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from '@opencode-ai/plugin/tui'
import { spawn, type ChildProcess } from 'node:child_process'
import path from 'node:path'
import { For, Show, createSignal, onCleanup } from 'solid-js'
import { colors } from '../shared/colors.ts'
import { icons } from '../shared/icons.ts'
import { SidebarSection } from '../shared/sidebar-section.tsx'

const id = 'opencode-markdown-context'
const MAX_LABEL_LENGTH = 30
const MAX_ROOT_LENGTH = 8
const MAX_PARENT_LENGTH = 12
const MIN_LEAF_LENGTH = 6

type MarkdownSourceKind = 'readme' | 'agents' | 'partial' | 'spec' | 'markdown'

type MarkdownContextItem = {
  key: string
  path: string
  label: string
  kind: MarkdownSourceKind
  compacted: boolean
  time: number
}

function MarkdownContext(props: { api: TuiPluginApi; sessionID: string }) {
  const [revision, setRevision] = createSignal(0)
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
      <SidebarSection api={props.api} title="Markdown Context" detail={`${items().length} read`}>
        <For each={items()}>
          {(item) => (
            <box flexDirection="row" gap={0} onMouseDown={() => openMarkdown(props.api, item.path)}>
              <text fg={sourceColor(props.api, item)} wrapMode="none">
                {sourceIcon(item)}
              </text>
              <text fg={props.api.theme.current.textMuted} wrapMode="none">
                {item.label}
              </text>
            </box>
          )}
        </For>
      </SidebarSection>
    </Show>
  )
}

function openMarkdown(api: TuiPluginApi, filePath: string) {
  let child: ChildProcess
  try {
    child = spawn('hyprd', ['tab', 'nvim', '--', filePath], { detached: true, stdio: 'ignore' })
  } catch {
    api.ui.toast({
      variant: 'warning',
      title: 'Markdown open failed',
      message: filePath,
    })
    return
  }

  child.once('error', () => {
    api.ui.toast({
      variant: 'warning',
      title: 'Markdown open failed',
      message: filePath,
    })
  })
  child.once('close', (code) => {
    if (code === 0) return
    api.ui.toast({
      variant: 'warning',
      title: 'Markdown open failed',
      message: `hyprd exited ${code ?? 'without a status'}: ${filePath}`,
    })
  })
  child.unref()
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

  return Array.from(reads.values()).sort((left, right) => right.time - left.time)
}

function markdownReadItem(api: TuiPluginApi, part: ReturnType<TuiPluginApi['state']['part']>[number]): MarkdownContextItem | undefined {
  if (part.type !== 'tool' || !isReadTool(part.tool)) return undefined
  const tool = part as ToolPart
  if (tool.state.status !== 'completed') return undefined

  const filePath = markdownPathFromInput(tool.state.input)
  if (!filePath) return undefined
  const kind = markdownSourceKind(filePath)

  return {
    key: filePath,
    path: filePath,
    label: displayPath(api, filePath, kind),
    kind,
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

function markdownSourceKind(filePath: string): MarkdownSourceKind {
  const normalizedPath = path.normalize(filePath)
  const leaf = path.basename(normalizedPath).toLowerCase()

  if (normalizedPath.split(/[\\/]/u).includes('.spec')) return 'spec'
  if (leaf === 'readme.md') return 'readme'
  if (leaf === 'agents.md') return 'agents'
  if (/^[A-Z][A-Z0-9_-]*\.md$/.test(path.basename(normalizedPath))) return 'partial'
  return 'markdown'
}

function normalizeFilePath(value: string) {
  const clean = value.replace(/^file:\/\//, '').split(/[?#]/, 1)[0]
  if (clean.startsWith('~/')) return path.join(process.env.HOME || '~', clean.slice(2))
  return clean
}

function displayPath(api: TuiPluginApi, filePath: string, kind: MarkdownSourceKind) {
  return compactPath(contextLabel(api, filePath, kind), kind)
}

function relativePath(api: TuiPluginApi, filePath: string) {
  const cwd = api.state.path.directory || api.state.path.worktree || ''
  const home = process.env.HOME || ''

  if (cwd && filePath.startsWith(cwd + path.sep)) return filePath.slice(cwd.length + 1)
  if (home && filePath.startsWith(home + path.sep)) return filePath.slice(home.length + 1)
  return filePath
}

function contextLabel(api: TuiPluginApi, filePath: string, kind: MarkdownSourceKind) {
  const label = relativePath(api, filePath)

  if (kind === 'readme' || kind === 'agents') {
    const dir = path.dirname(label)
    return dir === '.' ? contextRootName(api, filePath) : dir
  }

  return stripMarkdownExtension(label)
}

function contextRootName(api: TuiPluginApi, filePath: string) {
  const root = api.state.path.worktree || api.state.path.directory || path.dirname(filePath)
  return path.basename(root) || path.basename(path.dirname(filePath)) || path.basename(filePath)
}

function stripMarkdownExtension(label: string) {
  return label.replace(/\.(md|mdx|markdown)$/i, '')
}

function compactPath(label: string, kind: MarkdownSourceKind) {
  const parts = label.split(path.sep).filter(Boolean)
  if (parts.length <= 2) return label.length <= MAX_LABEL_LENGTH ? label : truncateLabel(label)

  const leaf = parts.at(-1) ?? label
  const parent = parts.at(-2) ?? ''
  const root = parts[0]

  if (kind === 'readme' || kind === 'agents') return compactRootLeaf(root, leaf)
  return compactRootParentLeaf(root, parent, leaf)
}

function truncateLabel(label: string) {
  if (label.length <= MAX_LABEL_LENGTH) return label
  return `${label.slice(0, Math.max(0, MAX_LABEL_LENGTH - 3))}...`
}

function compactRootLeaf(root: string, leaf: string) {
  const full = `${root}/.../${leaf}`
  if (full.length <= MAX_LABEL_LENGTH) return full

  const segmentBudget = MAX_LABEL_LENGTH - '/.../'.length
  if (segmentBudget < 2) return truncateLabel(full)

  const rootLength = Math.min(root.length, Math.max(1, Math.min(MAX_ROOT_LENGTH, segmentBudget - MIN_LEAF_LENGTH)))
  const leafLength = segmentBudget - rootLength
  const candidate = `${truncateMiddle(root, rootLength)}/.../${truncateFileName(leaf, leafLength)}`

  if (candidate.length <= MAX_LABEL_LENGTH) return candidate
  return truncateLabel(full)
}

function compactRootParentLeaf(root: string, parent: string, leaf: string) {
  const full = `${root}/.../${parent}/${leaf}`
  if (full.length <= MAX_LABEL_LENGTH) return full

  const segmentBudget = MAX_LABEL_LENGTH - '/...//'.length
  if (segmentBudget < 3) return truncateLabel(full)

  const rootLength = Math.min(root.length, Math.max(1, Math.min(MAX_ROOT_LENGTH, segmentBudget - MIN_LEAF_LENGTH)))
  const remaining = segmentBudget - rootLength
  const parentLength = Math.min(parent.length, Math.max(1, Math.min(MAX_PARENT_LENGTH, remaining - MIN_LEAF_LENGTH)))
  const leafLength = remaining - parentLength
  const candidate = `${truncateMiddle(root, rootLength)}/.../${truncateMiddle(parent, parentLength)}/${truncateFileName(leaf, leafLength)}`

  if (candidate.length <= MAX_LABEL_LENGTH) return candidate
  return truncateLabel(full)
}

function truncateMiddle(value: string, maxLength: number) {
  if (maxLength <= 0) return ''
  if (value.length <= maxLength) return value
  if (maxLength <= 3) return '.'.repeat(maxLength)

  const headLength = Math.ceil((maxLength - 3) / 2)
  const tailLength = Math.floor((maxLength - 3) / 2)
  return `${value.slice(0, headLength)}...${value.slice(value.length - tailLength)}`
}

function truncateFileName(value: string, maxLength: number) {
  if (value.length <= maxLength) return value

  const ext = path.extname(value)
  if (ext.length > 1 && maxLength > ext.length) {
    const stemLength = Math.min(3, maxLength - ext.length)
    return `${value.slice(0, stemLength)}${ext}`
  }

  return truncateMiddle(value, maxLength)
}

function sourceColor(api: TuiPluginApi, item: MarkdownContextItem) {
  const c = colors(api.theme.current)
  if (item.compacted) return c.red

  switch (item.kind) {
    case 'readme':
      return c.green
    case 'agents':
      return c.blue
    case 'partial':
      return c.yellow
    case 'spec':
      return c.cyan
    case 'markdown':
      return c.muted
  }
}

function sourceIcon(item: MarkdownContextItem) {
  if (item.compacted) return 'C '

  switch (item.kind) {
    case 'readme':
      return 'R '
    case 'agents':
      return 'A '
    case 'partial':
      return 'I '
    case 'spec':
      return `${icons.spec} `
    case 'markdown':
      return 'M '
  }
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
