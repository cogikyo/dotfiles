// @ts-nocheck -- Bun socket types are not available in this Node-typed opencode tsconfig.
const SOCKET_PATH = "/tmp/hyprd.sock"

export async function send(command) {
  let resolveDone
  let response = ""
  const done = new Promise((r) => {
    resolveDone = r
  })
  try {
    await Bun.connect({
      unix: SOCKET_PATH,
      socket: {
        open(socket) {
          socket.write(command)
        },
        data(_socket, data) {
          response += new TextDecoder().decode(data)
        },
        close() {
          resolveDone()
        },
        error() {
          resolveDone()
        },
      },
    })
  } catch {
    resolveDone()
    return false
  }
  await done
  return response.trim() === "ok"
}
