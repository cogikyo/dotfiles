import type { TuiPluginApi } from '@opencode-ai/plugin/tui'
import { spawn } from 'node:child_process'

export function openInNvim(api: TuiPluginApi, filePath: string, title: string) {
  const child = spawn('hyprd', ['edit', filePath], {
    detached: true,
    stdio: ['ignore', 'ignore', 'pipe'],
  })

  let stderr = ''
  child.stderr?.on('data', (chunk) => {
    stderr += String(chunk)
  })

  child.once('error', (error) => {
    api.ui.toast({
      variant: 'warning',
      title,
      message: error.message || filePath,
    })
  })
  child.once('close', (code) => {
    if (code === 0) return
    api.ui.toast({
      variant: 'warning',
      title,
      message: stderr.trim() || `hyprd exited ${code ?? 'without a status'}: ${filePath}`,
    })
  })
  child.unref()
}
