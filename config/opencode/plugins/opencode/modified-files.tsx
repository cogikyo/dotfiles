/** @jsxImportSource @opentui/solid */
import type { ToolPart } from '@opencode-ai/sdk/v2'
import type { TuiPlugin, TuiPluginApi, TuiPluginModule, TuiSidebarFileItem } from '@opencode-ai/plugin/tui'
import path from 'node:path'
import { For, Show, createSignal, onCleanup } from 'solid-js'
import { openInNvim } from '../shared/open-nvim.ts'
import { SidebarSection } from '../shared/sidebar-section.tsx'

const id = 'opencode-modified-files'
const MAX_PATH_LENGTH = 34

type FileItem = TuiSidebarFileItem & {
  path: string
  label: string
}

function ModifiedFiles(props: { api: TuiPluginApi; sessionID: string }) {
  const [revision, setRevision] = createSignal(0)
  const refresh = () => setRevision((value) => value + 1)

  const disposers = [
    props.api.event.on('session.diff', (event) => {
      if (event.properties.sessionID === props.sessionID) refresh()
    }),
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
  ]
  onCleanup(() => {
    for (const dispose of disposers) dispose()
  })

  const items = () => {
    revision()
    return modifiedFiles(props.api, props.sessionID)
  }

  return (
    <Show when={items().length > 0}>
      <SidebarSection api={props.api} title="Modified Files" detail={fileCount(items().length)}>
        <For each={items()}>
          {(item) => (
            <box flexDirection="row" gap={0} onMouseDown={() => openInNvim(props.api, item.path, 'Modified file open failed')}>
              <text fg={props.api.theme.current.textMuted} wrapMode="none">
                {item.label}
              </text>
              <Show when={item.additions > 0}>
                <text fg={props.api.theme.current.diffAdded} wrapMode="none">
                  {` +${item.additions}`}
                </text>
              </Show>
              <Show when={item.deletions > 0}>
                <text fg={props.api.theme.current.diffRemoved} wrapMode="none">
                  {` -${item.deletions}`}
                </text>
              </Show>
            </box>
          )}
        </For>
      </SidebarSection>
    </Show>
  )
}

function modifiedFiles(api: TuiPluginApi, sessionID: string): FileItem[] {
  const diffs = api.state.session.diff(sessionID)
  if (diffs.length > 0) return diffs.map((item) => fileItem(api, item.file, item.additions, item.deletions))
  return editedFiles(api, sessionID)
}

function editedFiles(api: TuiPluginApi, sessionID: string): FileItem[] {
  const seen = new Map<string, FileItem>()

  for (const message of api.state.session.messages(sessionID)) {
    for (const part of api.state.part(message.id)) {
      if (part.type !== 'tool' || !isEditTool(part.tool)) continue
      const tool = part as ToolPart
      if (tool.state.status !== 'completed') continue

      for (const filePath of editPaths(tool)) {
        const item = fileItem(api, filePath, 0, 0)
        seen.set(item.path, item)
      }
    }
  }

  return Array.from(seen.values())
}

function fileItem(api: TuiPluginApi, file: string, additions: number, deletions: number): FileItem {
  const filePath = rootedPath(api, file)
  return {
    file,
    additions,
    deletions,
    path: filePath,
    label: compactPath(relativePath(api, filePath)),
  }
}

function isEditTool(tool: string) {
  return tool === 'edit' || tool === 'write' || tool === 'apply_patch' || tool === 'Edit' || tool === 'Write' || tool === 'ApplyPatch'
}

function editPaths(tool: ToolPart) {
  if (tool.state.status !== 'completed') return []

  const paths: string[] = []
  const input = tool.state.input
  for (const key of ['filePath', 'path', 'filepath', 'file']) {
    const value = input[key]
    if (typeof value === 'string' && value) paths.push(value)
  }

  const patch = input.patch ?? input.patchText
  if (typeof patch === 'string') {
    for (const match of patch.matchAll(/^\*\*\* (?:Add|Update|Delete) File: (.+)$/gm)) {
      const filePath = match[1]?.trim()
      if (filePath) paths.push(filePath)
    }
  }

  return paths
}

function fileCount(count: number) {
  return `${count} ${count === 1 ? 'file' : 'files'}`
}

function rootedPath(api: TuiPluginApi, filePath: string) {
  if (path.isAbsolute(filePath)) return filePath
  return path.join(api.state.path.worktree || api.state.path.directory || process.cwd(), filePath)
}

function relativePath(api: TuiPluginApi, filePath: string) {
  const root = api.state.path.worktree || api.state.path.directory || ''
  if (root && filePath.startsWith(root + path.sep)) return filePath.slice(root.length + 1)
  return filePath
}

function compactPath(filePath: string) {
  if (filePath.length <= MAX_PATH_LENGTH) return filePath

  const parts = filePath.split(path.sep).filter(Boolean)
  if (parts.length <= 2) return truncateMiddle(filePath, MAX_PATH_LENGTH)

  const leaf = parts.at(-1) ?? filePath
  const parent = parts.at(-2) ?? ''
  const root = parts[0]
  const label = `${root}/.../${parent}/${leaf}`
  if (label.length <= MAX_PATH_LENGTH) return label

  return truncateMiddle(label, MAX_PATH_LENGTH)
}

function truncateMiddle(value: string, maxLength: number) {
  if (value.length <= maxLength) return value
  if (maxLength <= 3) return '.'.repeat(maxLength)

  const headLength = Math.ceil((maxLength - 3) / 2)
  const tailLength = Math.floor((maxLength - 3) / 2)
  return `${value.slice(0, headLength)}...${value.slice(value.length - tailLength)}`
}

const tui: TuiPlugin = async (api) => {
  api.slots.register({
    order: 1000,
    slots: {
      sidebar_content(_ctx, props: { session_id: string }) {
        return <ModifiedFiles api={api} sessionID={props.session_id} />
      },
    },
  })
}

const plugin: TuiPluginModule & { id: string } = {
  id,
  tui,
}

export default plugin
