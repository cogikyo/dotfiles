/** @jsxImportSource @opentui/solid */
import type { Message, ToolPart } from '@opencode-ai/sdk/v2'
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from '@opencode-ai/plugin/tui'
import { existsSync, realpathSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { For, Show, createSignal, onCleanup } from 'solid-js'
import { colors } from '../shared/colors.ts'
import { icons } from '../shared/icons.ts'
import { openInNvim } from '../shared/open-nvim.ts'
import { SidebarSection } from '../shared/sidebar-section.tsx'

const id = 'opencode-markdown-context'
const MAX_LABEL_LENGTH = 30
const MAX_ROOT_LENGTH = 8
const MAX_PARENT_LENGTH = 12
const MIN_LEAF_LENGTH = 6

type MarkdownSourceKind = 'readme' | 'agents' | 'agent' | 'skill' | 'partial' | 'spec' | 'markdown'

const configRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..')

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
            <box flexDirection="row" gap={0} onMouseDown={() => openInNvim(props.api, item.path, 'Markdown open failed')}>
              <text fg={sourceColor(props.api, item)} wrapMode="none">
                {sourceIcon(props.api, item)}
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

function markdownContextItems(api: TuiPluginApi, sessionID: string) {
  const pinned = pinnedContextItems(api, sessionID)
  const seen = new Set(pinned.map((item) => item.key))
  const reads = new Map<string, MarkdownContextItem>()
  const messages = api.state.session.messages(sessionID) as ReadonlyArray<Message>

  for (const message of messages) {
    for (const part of api.state.part(message.id)) {
      const item = skillToolItem(api, part) ?? markdownReadItem(api, part)
      if (!item) continue

      const pin = pinned.find((entry) => entry.key === item.key)
      if (pin) {
        if (isConfigAgents(item.path)) pin.compacted = item.compacted
        continue
      }
      if (seen.has(item.key)) continue

      const existing = reads.get(item.key)
      if (!existing || item.time >= existing.time) reads.set(item.key, item)
    }
  }

  return [...pinned, ...Array.from(reads.values()).sort((left, right) => right.time - left.time)]
}

function pinnedContextItems(api: TuiPluginApi, sessionID: string) {
  const items: MarkdownContextItem[] = []
  const seen = new Set<string>()

  const push = (filePath: string) => {
    if (!existsSync(filePath)) return
    const item = markdownFileItem(api, filePath, 0, false)
    if (seen.has(item.key)) return
    seen.add(item.key)
    items.push(item)
  }

  push(path.join(configRoot, 'AGENTS.md'))
  const projectRoot = api.state.path.worktree || api.state.path.directory
  if (projectRoot) push(path.join(projectRoot, 'AGENTS.md'))

  const agent = currentAgent(api, sessionID)
  if (agent) push(path.join(configRoot, 'agents', `${agent}.md`))

  return items
}

function currentAgent(api: TuiPluginApi, sessionID: string) {
  const messages = api.state.session.messages(sessionID) as ReadonlyArray<Message>
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index]
    if ('agent' in message && typeof message.agent === 'string' && message.agent) return message.agent
  }
  return undefined
}

function markdownReadItem(api: TuiPluginApi, part: ReturnType<TuiPluginApi['state']['part']>[number]): MarkdownContextItem | undefined {
  if (part.type !== 'tool' || !isReadTool(part.tool)) return undefined
  const tool = part as ToolPart
  if (tool.state.status !== 'completed') return undefined

  const filePath = markdownPathFromInput(tool.state.input)
  if (!filePath) return undefined
  return markdownFileItem(api, filePath, tool.state.time.end, tool.state.time.compacted !== undefined)
}

function skillToolItem(api: TuiPluginApi, part: ReturnType<TuiPluginApi['state']['part']>[number]): MarkdownContextItem | undefined {
  if (part.type !== 'tool' || !isSkillTool(part.tool)) return undefined
  const tool = part as ToolPart
  if (tool.state.status !== 'completed') return undefined

  const filePath = skillPathFromTool(tool)
  if (!filePath) return undefined
  return markdownFileItem(api, filePath, tool.state.time.end, tool.state.time.compacted !== undefined)
}

function skillPathFromTool(tool: ToolPart) {
  if (tool.state.status !== 'completed') return undefined
  const dir = tool.state.metadata.dir
  if (typeof dir === 'string' && dir) {
    const filePath = path.join(dir, 'SKILL.md')
    if (existsSync(filePath)) return filePath
  }

  const name = tool.state.input.name
  if (typeof name !== 'string' || !name) return undefined
  const filePath = path.join(configRoot, 'skills', name, 'SKILL.md')
  return existsSync(filePath) ? filePath : undefined
}

function markdownFileItem(api: TuiPluginApi, filePath: string, time: number, compacted: boolean): MarkdownContextItem {
  const kind = markdownSourceKind(filePath)
  return {
    key: markdownIdentity(filePath),
    path: filePath,
    label: displayPath(api, filePath, kind),
    kind,
    compacted,
    time,
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

function isSkillTool(tool: string) {
  return tool === 'skill' || tool === 'Skill'
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
  if (leaf === 'skill.md') return 'skill'
  if (agentSegments(normalizedPath)) return 'agent'
  if (/^[A-Z][A-Z0-9_-]*\.md$/.test(path.basename(normalizedPath))) return 'partial'
  return 'markdown'
}

function agentSegments(filePath: string) {
  const parts = path.normalize(filePath).split(/[\\/]/u).filter(Boolean)
  const index = parts.findIndex((part) => part === 'agents' || part === 'agent')
  if (index === -1) return undefined
  const rest = parts.slice(index + 1)
  if (rest.length === 0 || !isMarkdownPath(rest.at(-1) ?? '')) return undefined
  return rest
}

function agentLabel(filePath: string) {
  const rest = agentSegments(filePath)
  if (!rest) return stripMarkdownExtension(path.basename(filePath))
  return stripMarkdownExtension(rest.join('/'))
    .split('/')
    .map(titleSegment)
    .join('/')
}

function isSubagent(filePath: string) {
  return (agentSegments(filePath)?.length ?? 0) > 1
}

function titleSegment(value: string) {
  return value ? value.charAt(0).toUpperCase() + value.slice(1) : value
}

function isGlobalOpencodePath(filePath: string) {
  const file = markdownIdentity(filePath)
  const root = markdownIdentity(configRoot)
  return file === root || file.startsWith(root + path.sep)
}

function skillName(filePath: string) {
  const parent = path.basename(path.dirname(filePath))
  if (!parent || parent === '.' || parent === 'skills' || parent === 'skill') return 'Skill'
  return parent.split('-').map(titleSegment).join('-')
}

function skillProjectOwner(api: TuiPluginApi, filePath: string) {
  const parts = path.normalize(filePath).split(/[\\/]/u).filter(Boolean)
  const skillsIndex = parts.reduce((found, part, index) => (part === 'skills' || part === 'skill' ? index : found), -1)
  if (skillsIndex > 0) {
    let ownerIndex = skillsIndex - 1
    if (parts[ownerIndex] === '.opencode' || parts[ownerIndex] === 'opencode') ownerIndex -= 1
    const owner = parts[ownerIndex]
    if (owner) return owner.replace(/^\./, '')
  }
  return contextRootName(api, filePath)
}

function skillLabel(api: TuiPluginApi, filePath: string) {
  const name = skillName(filePath)
  if (isGlobalOpencodePath(filePath)) return name
  return `${skillProjectOwner(api, filePath)}/${name}`
}

function normalizeFilePath(value: string) {
  const clean = value.replace(/^file:\/\//, '').split(/[?#]/, 1)[0]
  if (clean.startsWith('~/')) return path.join(process.env.HOME || '~', clean.slice(2))
  return clean
}

function markdownIdentity(filePath: string) {
  const normalizedPath = path.normalize(filePath)
  try {
    return realpathSync.native(normalizedPath)
  } catch {
    return normalizedPath
  }
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

  if (kind === 'spec') return specLabel(filePath)
  if (kind === 'agent') return agentLabel(filePath)
  if (kind === 'skill') return skillLabel(api, filePath)

  if (kind === 'readme' || kind === 'agents') {
    if (kind === 'agents' && isConfigAgents(filePath)) return 'OpenCode'
    const dir = path.dirname(label)
    return dir === '.' ? contextRootName(api, filePath) : dir
  }

  return stripMarkdownExtension(label)
}

function specLabel(filePath: string) {
  const parts = path.normalize(filePath).split(/[\\/]/u).filter(Boolean)
  const specIndex = parts.lastIndexOf('.spec')
  const owner = parts[specIndex - 1]
  const nestedPath = parts.slice(specIndex + 1)

  return stripMarkdownExtension([owner, ...nestedPath].filter(Boolean).join(path.sep))
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
      return isConfigAgents(item.path) ? c.cyan : c.blue
    case 'agent':
      return isSubagent(item.path) ? c.magenta : c.blue
    case 'skill':
      return isGlobalOpencodePath(item.path) ? c.orange : c.pink
    case 'partial':
      return c.yellow
    case 'spec':
      return c.cyan
    case 'markdown':
      return c.muted
  }
}

function isConfigAgents(filePath: string) {
  return path.basename(filePath).toLowerCase() === 'agents.md' && isGlobalOpencodePath(filePath)
}

function isRootAgents(api: TuiPluginApi, filePath: string) {
  const root = api.state.path.worktree || api.state.path.directory
  if (!root) return false
  return path.normalize(path.dirname(filePath)) === path.normalize(root)
}

function sourceIcon(api: TuiPluginApi, item: MarkdownContextItem) {
  if (item.compacted) return `${icons.compacted} `

  switch (item.kind) {
    case 'readme':
      return `${icons.readme} `
    case 'agents':
      if (isConfigAgents(item.path)) return `${icons.agentsCore} `
      return `${isRootAgents(api, item.path) ? icons.folderLibrary : icons.folder} `
    case 'agent':
      return `${isSubagent(item.path) ? icons.subagent : icons.agents} `
    case 'skill':
      return `${isGlobalOpencodePath(item.path) ? icons.skill : icons.skillProject} `
    case 'partial':
      return `${icons.partial} `
    case 'spec':
      return `${icons.spec} `
    case 'markdown':
      return `${icons.markdown} `
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
