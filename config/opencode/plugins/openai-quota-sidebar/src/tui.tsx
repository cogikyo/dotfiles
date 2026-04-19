/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from '@opencode-ai/plugin/tui'
import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import { createSignal, For, onCleanup, Show } from 'solid-js'

const id = 'cullyn.openai-quota-sidebar'
const INTERNAL_CONTEXT_PLUGIN_ID = 'internal:sidebar-context'
const REFRESH_MS = 60_000
const BAR_WIDTH = 10

type AuthFile = {
  openai?: {
    type?: string
    access?: string
    accountId?: string
  }
}

type UsageWindow = {
  label: string
  remainingPercent: number
  resetAt?: string
}

type ResetParts = {
  duration: string
  exact: string
}

type SessionProvider = {
  providerID?: string
}

type QuotaState = {
  windows: UsageWindow[]
  note?: string
}

function resolveOpencodeDataDir() {
  const xdg = process.env.XDG_DATA_HOME?.trim()
  if (xdg) return path.join(path.resolve(xdg), 'opencode')
  return path.join(os.homedir(), '.local', 'share', 'opencode')
}

function authPath() {
  return path.join(resolveOpencodeDataDir(), 'auth.json')
}

function decodeJwtPayload(token: string) {
  const parts = token.split('.')
  if (parts.length !== 3) return undefined

  try {
    return JSON.parse(Buffer.from(parts[1], 'base64url').toString('utf8')) as {
      'https://api.openai.com/auth'?: {
        chatgpt_account_id?: string
      }
    }
  } catch {
    return undefined
  }
}

function accountIDFromToken(token: string) {
  return decodeJwtPayload(token)?.['https://api.openai.com/auth']?.chatgpt_account_id
}

function normalizePercent(value: unknown) {
  if (typeof value !== 'number' || Number.isNaN(value)) return undefined
  const expanded = value > 0 && value < 1 ? value * 100 : value
  return Math.max(0, Math.min(100, expanded))
}

function resetAtFromWindow(window: Record<string, unknown>, fallback?: Record<string, unknown>) {
  const absolute =
    typeof window.reset_at === 'string'
      ? window.reset_at
      : typeof fallback?.reset_at === 'string'
        ? fallback.reset_at
        : undefined
  if (absolute) return absolute

  const resetAfterSeconds =
    typeof window.reset_after_seconds === 'number'
      ? window.reset_after_seconds
      : typeof fallback?.reset_after_seconds === 'number'
        ? fallback.reset_after_seconds
        : undefined
  if (resetAfterSeconds === undefined || resetAfterSeconds < 0) return undefined
  return new Date(Date.now() + resetAfterSeconds * 1000).toISOString()
}

function formatReset(resetAt?: string): ResetParts {
  if (!resetAt) return { duration: '--', exact: '--' }
  const resetDate = new Date(resetAt)
  const ms = resetDate.getTime() - Date.now()
  if (!Number.isFinite(ms)) return { duration: '--', exact: '--' }
  if (ms <= 0) return { duration: 'now', exact: 'now' }

  const totalMinutes = Math.ceil(ms / 60_000)
  const days = Math.floor(totalMinutes / (24 * 60))
  const hours = Math.floor((totalMinutes % (24 * 60)) / 60)
  const minutes = totalMinutes % 60

  const duration =
    days > 0 ? `${days}d ${String(hours).padStart(2, '0')}h`
    : hours > 0 ? `${hours}h ${String(minutes).padStart(2, '0')}m`
    : `${String(minutes).padStart(2, '0')}m`

  const now = new Date()
  const sameDay = resetDate.toDateString() === now.toDateString()
  const time = new Intl.DateTimeFormat(undefined, {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(resetDate)
  const date = new Intl.DateTimeFormat(undefined, {
    month: 'numeric',
    day: 'numeric',
  }).format(resetDate)

  return {
    duration,
    exact: sameDay ? time : `${date} ${time}`,
  }
}

function providerIDForSession(api: TuiPluginApi, sessionID: string) {
  const messages = api.state.session.messages(sessionID) as ReadonlyArray<SessionProvider | {
    model?: SessionProvider
  }>

  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index]
    if ('providerID' in message && typeof message.providerID === 'string') {
      return message.providerID
    }

    if ('model' in message && typeof message.model?.providerID === 'string') {
      return message.model.providerID
    }
  }

  return undefined
}

function bar(percent: number) {
  const filled = Math.max(0, Math.min(BAR_WIDTH, Math.round((percent / 100) * BAR_WIDTH)))
  return '█'.repeat(filled) + '░'.repeat(BAR_WIDTH - filled)
}

function tone(api: TuiPluginApi, percent: number) {
  if (percent >= 60) return api.theme.current.success
  if (percent >= 25) return api.theme.current.warning
  return api.theme.current.error
}

async function loadQuotaState(): Promise<QuotaState> {
  const raw = await fs.readFile(authPath(), 'utf8')
  const auth = JSON.parse(raw) as AuthFile
  const openai = auth.openai

  if (!openai || openai.type !== 'oauth' || !openai.access) {
    return { windows: [], note: 'OpenAI OAuth not found' }
  }

  const accountID = openai.accountId || accountIDFromToken(openai.access)
  const headers = new Headers({
    Authorization: `Bearer ${openai.access}`,
    Accept: 'application/json',
    'User-Agent': 'openai-quota-sidebar',
  })
  if (accountID) headers.set('ChatGPT-Account-Id', accountID)

  const response = await fetch('https://chatgpt.com/backend-api/wham/usage', { headers })
  if (!response.ok) {
    return { windows: [], note: `OpenAI quota HTTP ${response.status}` }
  }

  const payload = (await response.json()) as {
    rate_limit?: {
      reset_at?: string
      reset_after_seconds?: number
      primary_window?: Record<string, unknown>
      secondary_window?: Record<string, unknown>
    }
  }
  const rateLimit = payload.rate_limit ?? {}
  const primaryWindow = rateLimit.primary_window ?? {}
  const secondaryWindow = rateLimit.secondary_window

  const windows: UsageWindow[] = []
  const primaryRemaining =
    normalizePercent(primaryWindow.remaining_percent) ??
    (() => {
      const used = normalizePercent(primaryWindow.used_percent)
      return used === undefined ? undefined : 100 - used
    })()
  if (primaryRemaining !== undefined) {
    windows.push({
      label: '5h',
      remainingPercent: primaryRemaining,
      resetAt: resetAtFromWindow(primaryWindow, rateLimit),
    })
  }

  if (secondaryWindow) {
    const secondaryRemaining =
      normalizePercent(secondaryWindow.remaining_percent) ??
      (() => {
        const used = normalizePercent(secondaryWindow.used_percent)
        return used === undefined ? undefined : 100 - used
      })()
    if (secondaryRemaining !== undefined) {
      windows.push({
        label: 'W',
        remainingPercent: secondaryRemaining,
        resetAt: resetAtFromWindow(secondaryWindow, rateLimit),
      })
    }
  }

  if (windows.length === 0) return { windows: [], note: 'Quota windows unavailable' }
  return { windows }
}

function QuotaPanel(props: { api: TuiPluginApi; sessionID: string }) {
  const [state, setState] = createSignal<QuotaState>({ windows: [] })
  const [isVisible, setIsVisible] = createSignal(false)

  const syncVisibility = () => {
    setIsVisible(providerIDForSession(props.api, props.sessionID) === 'openai')
  }

  const refresh = () => {
    syncVisibility()
    if (!isVisible()) {
      setState({ windows: [] })
      return
    }

    void loadQuotaState()
      .then((next) => setState(next))
      .catch((error: unknown) => {
        const message = error instanceof Error ? error.message : String(error)
        setState({ windows: [], note: message })
      })
  }

  refresh()
  const timer = setInterval(refresh, REFRESH_MS)
  const disposeMessageUpdated = props.api.event.on('message.updated', (event) => {
    if (event.properties.sessionID !== props.sessionID) return
    refresh()
  })
  const disposeMessageRemoved = props.api.event.on('message.removed', (event) => {
    if (event.properties.sessionID !== props.sessionID) return
    refresh()
  })
  const disposeSessionUpdated = props.api.event.on('session.updated', (event) => {
    if (event.properties.sessionID !== props.sessionID) return
    syncVisibility()
  })
  onCleanup(() => clearInterval(timer))
  onCleanup(disposeMessageUpdated)
  onCleanup(disposeMessageRemoved)
  onCleanup(disposeSessionUpdated)

  return (
    <Show when={isVisible()}>
      <box flexDirection="column" gap={0} paddingLeft={1}>
        <box flexDirection="row" gap={0}>
          <text fg={props.api.theme.current.text}>Quota</text>
          <text fg={props.api.theme.current.textMuted}> [OAI]</text>
        </box>
        <For each={state().windows}>
          {(window) => (
            <box flexDirection="row" gap={1}>
              <text fg={props.api.theme.current.textMuted}>{window.label.padEnd(2, ' ')}</text>
              <text fg={props.api.theme.current.text}>{String(Math.round(window.remainingPercent)).padStart(2, '0')}%</text>
              <text fg={tone(props.api, window.remainingPercent)}>{bar(window.remainingPercent)}</text>
              {(() => {
                const reset = formatReset(window.resetAt)
                return (
                  <>
                    <text fg={props.api.theme.current.textMuted}>{reset.duration}</text>
                    <box flexGrow={1} />
                    <text fg={props.api.theme.current.textMuted}>{reset.exact}</text>
                  </>
                )
              })()}
            </box>
          )}
        </For>
        <Show when={state().note}>
          <text fg={props.api.theme.current.error}>{state().note}</text>
        </Show>
      </box>
    </Show>
  )
}

const tui: TuiPlugin = async (api) => {
  let didDeactivateContext = false
  const contextPlugin = api.plugins
    .list()
    .find((item) => item.id === INTERNAL_CONTEXT_PLUGIN_ID)
  if (contextPlugin?.active) {
    didDeactivateContext = await api.plugins
      .deactivate(INTERNAL_CONTEXT_PLUGIN_ID)
      .catch(() => false)
  }

  api.lifecycle.onDispose(() => {
    if (!didDeactivateContext) return
    return api.plugins.activate(INTERNAL_CONTEXT_PLUGIN_ID).then(() => undefined)
  })

  api.slots.register({
    order: 100,
    slots: {
      sidebar_title() {
        return null
      },
      sidebar_content(_ctx, props: { session_id: string }) {
        return <QuotaPanel api={api} sessionID={props.session_id} />
      },
    },
  })
}

const plugin: TuiPluginModule & { id: string } = {
  id,
  tui,
}

export default plugin
