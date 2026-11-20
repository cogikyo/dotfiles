import { connect } from "node:net";
import { socketEnv, tokenEnv } from "./agy.ts";

const socket = connect(process.env[socketEnv] ?? "");
socket.write(`${process.env[tokenEnv] ?? ""}\n`);
process.stdin.pipe(socket);
socket.pipe(process.stdout);
socket.on("error", (error) => {
  console.error(`OpenCode tool bridge: ${error.message}`);
  process.exit(1);
});
socket.on("close", () => process.exit(0));
