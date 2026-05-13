import type { Message, Model, Provider } from '@opencode-ai/sdk/v2'
import type { TuiPluginApi } from '@opencode-ai/plugin/tui'

export type SessionMeta = {
  agent: string
  providerID: string
  providerName: string
  modelID: string
  modelName: string
  variant: string
  cwd: string
}

export type SessionUsage = {
  tokens: number
  limit?: number
  percent: number
  colorPercent: number
}

const AUTOCOMPACT_CONTEXT_LIMIT = 150_000

type AssistantLike = Extract<Message, { role: 'assistant' }>
type UserLike = Extract<Message, { role: 'user' }>

export function sessionMessages(api: TuiPluginApi, sessionID: string) {
  return api.state.session.messages(sessionID) as ReadonlyArray<Message>
}

export function sessionProviderID(api: TuiPluginApi, sessionID: string) {
  return providerIDFor(latestModelMessage(sessionMessages(api, sessionID)))
}

export function sessionMeta(api: TuiPluginApi, sessionID: string): SessionMeta {
  const messages = sessionMessages(api, sessionID)
  const latest = latestModelMessage(messages)
  const providerID = sessionProviderID(api, sessionID)
  const modelID = modelIDFor(latest)
  const model = findModel(api.state.provider, providerID, modelID)

  return {
    agent: title(latest?.agent || 'Build'),
    providerID,
    providerName: providerLabel(providerID),
    modelID,
    modelName: model?.name || modelLabel(modelID),
    variant: variantFor(latest),
    cwd: cwdFor(latest) || api.state.path.directory || api.state.path.worktree || '',
  }
}

export function sessionUsage(api: TuiPluginApi, sessionID: string): SessionUsage {
  const messages = sessionMessages(api, sessionID)
  const meta = sessionMeta(api, sessionID)
  const model = findModel(api.state.provider, meta.providerID, meta.modelID)
  const tokens = tokenTotal(latestAssistantMessage(messages))
  const limit = model?.limit.context
  const percent = limit ? Math.min(100, (tokens / limit) * 100) : 0

  return { tokens, limit, percent, colorPercent: percent }
}

export function sessionContextUsage(api: TuiPluginApi, sessionID: string): SessionUsage {
  const messages = sessionMessages(api, sessionID)
  const meta = sessionMeta(api, sessionID)
  const model = findModel(api.state.provider, meta.providerID, meta.modelID)
  const tokens = contextTokenTotal(latestAssistantMessage(messages))
  const limit = contextCompactionLimit(model?.limit.context)
  const percent = limit ? Math.min(100, (tokens / limit) * 100) : 0

  return { tokens, limit, percent, colorPercent: percent }
}

function contextCompactionLimit(modelLimit?: number) {
  if (!modelLimit) return undefined
  return Math.min(modelLimit, AUTOCOMPACT_CONTEXT_LIMIT)
}

function latestModelMessage(messages: ReadonlyArray<Message>): (AssistantLike | UserLike) | undefined {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index]
    if (message.role === 'assistant' || message.role === 'user') return message
  }
  return undefined
}

function latestAssistantMessage(messages: ReadonlyArray<Message>): AssistantLike | undefined {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index]
    if (message.role === 'assistant' && message.tokens.output > 0) return message
  }
  return undefined
}

function providerIDFor(message?: AssistantLike | UserLike) {
  if (!message) return ''
  return message.role === 'assistant' ? message.providerID : message.model.providerID
}

function modelIDFor(message?: AssistantLike | UserLike) {
  if (!message) return ''
  return message.role === 'assistant' ? message.modelID : message.model.modelID
}

function variantFor(message?: AssistantLike | UserLike) {
  if (!message) return ''
  return message.role === 'assistant' ? message.variant || '' : message.model.variant || ''
}

function cwdFor(message?: AssistantLike | UserLike) {
  if (!message || message.role !== 'assistant') return ''
  return message.path.cwd
}

function findModel(providers: ReadonlyArray<Provider>, providerID: string, modelID: string): Model | undefined {
  if (!providerID || !modelID) return undefined
  return providers.find((provider) => provider.id === providerID)?.models[modelID]
}

function tokenTotal(message?: AssistantLike) {
  if (!message) return 0
  const tokens = message.tokens
  return tokens.input + tokens.output + tokens.reasoning + tokens.cache.read + tokens.cache.write
}

function contextTokenTotal(message?: AssistantLike) {
  if (!message) return 0
  const tokens = message.tokens
  return tokens.input + tokens.cache.read + tokens.cache.write
}

function providerLabel(providerID: string) {
  if (providerID === 'openai') return 'OpenAI'
  if (providerID === 'anthropic') return 'Claude'
  return title(providerID)
}

function modelLabel(modelID: string) {
  return modelID
    .replace(/^claude-/, 'Claude ')
    .replace(/^gpt-/, 'GPT-')
    .split(/[-_]/)
    .filter(Boolean)
    .map((part) => (/^\d/.test(part) || part.toUpperCase() === part ? part : title(part)))
    .join(' ')
}

function title(value: string) {
  if (!value) return ''
  return value.charAt(0).toUpperCase() + value.slice(1)
}

export function shortDir(dir: string) {
  if (!dir) return ''
  const home = process.env.HOME
  if (home && dir === home) return '~'
  if (home && dir.startsWith(home + '/')) return '~/' + dir.slice(home.length + 1)
  return dir
}

export function formatTokens(tokens: number) {
  if (tokens >= 1_000_000) return `${trim(tokens / 1_000_000)}M`
  if (tokens >= 1_000) return `${trim(tokens / 1_000)}K`
  return String(tokens)
}

function trim(value: number) {
  return value >= 100 ? value.toFixed(0) : value >= 10 ? value.toFixed(1) : value.toFixed(2)
}
