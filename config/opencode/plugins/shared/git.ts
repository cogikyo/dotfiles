import { execFile } from 'node:child_process'
import { promisify } from 'node:util'

const execFileAsync = promisify(execFile)

export type GitStatus = {
  branch: string
  ahead: number
  behind: number
  staged: number
  modified: number
  untracked: number
  deleted: number
  stashed: number
  renamed: number
  conflicted: number
  complete: boolean
}

export async function gitStatus(dir?: string): Promise<GitStatus | undefined> {
  if (!dir) return undefined

  try {
    await execFileAsync('git', ['-C', dir, 'rev-parse', '--git-dir'], { timeout: 750 })
    const { stdout } = await execFileAsync(
      'git',
      ['-C', dir, '--no-optional-locks', 'status', '--porcelain=v2', '--branch', '--show-stash'],
      { timeout: 1000, maxBuffer: 256 * 1024 },
    )
    return parseGitStatus(stdout)
  } catch {
    return undefined
  }
}

function parseGitStatus(output: string): GitStatus {
  const status: GitStatus = {
    branch: '',
    ahead: 0,
    behind: 0,
    staged: 0,
    modified: 0,
    untracked: 0,
    deleted: 0,
    stashed: 0,
    renamed: 0,
    conflicted: 0,
    complete: true,
  }

  for (const line of output.split('\n')) {
    if (!line) continue

    if (line.startsWith('# branch.head ')) {
      status.branch = line.slice('# branch.head '.length)
      continue
    }

    if (line.startsWith('# branch.ab ')) {
      const parts = line.split(/\s+/)
      status.ahead = Number.parseInt(parts[2]?.slice(1) ?? '0', 10) || 0
      status.behind = Number.parseInt(parts[3]?.slice(1) ?? '0', 10) || 0
      continue
    }

    if (line.startsWith('# stash ')) {
      status.stashed = Number.parseInt(line.slice('# stash '.length), 10) || 0
      continue
    }

    if (line.startsWith('#') || line.length < 4) continue

    const entryType = line[0]
    const x = line[2]
    const y = line[3]

    if (entryType === '?') {
      status.untracked++
      continue
    }

    if (entryType === 'u') {
      status.conflicted++
      continue
    }

    if (entryType === '1') {
      if (x !== '.' && x !== ' ') status.staged++
      if (y === 'M' || y === 'T') status.modified++
      if (y === 'D') status.deleted++
      continue
    }

    if (entryType === '2') status.renamed++
  }

  return status
}

export function gitDirtyCount(status: GitStatus) {
  return status.staged + status.modified + status.untracked + status.deleted + status.renamed + status.conflicted
}
