const SOCKET_PATH = "/tmp/hyprd.sock";

/** Sends one hyprd socket command; only an `ok` reply succeeds, and a missing close has no timeout. */
export async function send(command: string) {
  let response = "";
  const done = Promise.withResolvers<void>();
  try {
    await Bun.connect({
      unix: SOCKET_PATH,
      socket: {
        open(socket) {
          socket.write(command);
        },
        data(_socket, data) {
          response += new TextDecoder().decode(data);
        },
        close() {
          done.resolve();
        },
        error() {
          done.resolve();
        },
      },
    });
  } catch {
    return false;
  }
  await done.promise;
  return response.trim() === "ok";
}
