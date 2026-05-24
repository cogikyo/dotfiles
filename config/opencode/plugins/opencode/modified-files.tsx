/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginApi, TuiPluginModule, TuiSidebarFileItem } from '@opencode-ai/plugin/tui'
import { spawn, type ChildProcess } from 'node:child_process'
import path from 'node:path'
import { For, Show, createSignal, onCleanup } from 'solid-js'
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

  const dispose = props.api.event.on('session.diff', (event) => {
    if (event.properties.sessionID === props.sessionID) refresh()
  })
  onCleanup(dispose)

  const items = () => {
    revision()
    return modifiedFiles(props.api, props.sessionID)
  }

  return (
    <Show when={items().length > 0}>
      <SidebarSection api={props.api} title="Modified Files" detail={fileCount(items().length)}>
        <For each={items()}>
          {(item) => (
            <box flexDirection="row" gap={0} onMouseDown={() => openFile(props.api, item.path)}>
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
  return api.state.session.diff(sessionID).map((item) => {
    const filePath = rootedPath(api, item.file)
    return {
      ...item,
      path: filePath,
      label: compactPath(relativePath(api, filePath)),
    }
  })
}

function fileCount(count: number) {
  return `${count} ${count === 1 ? 'file' : 'files'}`
}

function openFile(api: TuiPluginApi, filePath: string) {
  let child: ChildProcess
  try {
    child = spawn('hyprd', ['tab', 'nvim', '--', filePath], { detached: true, stdio: 'ignore' })
  } catch {
    api.ui.toast({
      variant: 'warning',
      title: 'Modified file open failed',
      message: filePath,
    })
    return
  }

  child.once('error', () => {
    api.ui.toast({
      variant: 'warning',
      title: 'Modified file open failed',
      message: filePath,
    })
  })
  child.once('close', (code) => {
    if (code === 0) return
    api.ui.toast({
      variant: 'warning',
      title: 'Modified file open failed',
      message: `hyprd exited ${code ?? 'without a status'}: ${filePath}`,
    })
  })
  child.unref()
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
