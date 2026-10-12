import { appendFileSync, existsSync, mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { homedir } from "node:os";
const JWT_PATTERN = /^eyJ[A-Za-z0-9_-]{10,}/;
let mode = "disabled";
let logFilePath = null;
let logStream = null;
function getDefaultLogPath() {
    return join(homedir(), ".local", "share", "opencode", "claude-auth-debug.log");
}
export function initLogger(options) {
    closeLogger();
    if (options?.stream) {
        mode = "stream";
        logStream = options.stream;
        return;
    }
    const envVal = process.env.CLAUDE_AUTH_DEBUG;
    if (!envVal) {
        mode = "disabled";
        return;
    }
    mode = "file";
    logFilePath = envVal === "1" ? getDefaultLogPath() : envVal;
    const dir = dirname(logFilePath);
    if (!existsSync(dir)) {
        mkdirSync(dir, { recursive: true });
    }
    writeFileSync(logFilePath, "", "utf-8");
}
export function log(event, data) {
    if (mode === "disabled")
        return;
    const entry = {
        ts: new Date().toISOString(),
        event,
        ...redact(data ?? {}),
    };
    const line = JSON.stringify(entry) + "\n";
    if (mode === "file" && logFilePath) {
        appendFileSync(logFilePath, line, "utf-8");
    }
    else if (mode === "stream" && logStream) {
        logStream.write(line);
    }
}
export function closeLogger() {
    mode = "disabled";
    logFilePath = null;
    logStream = null;
}
function redactValue(key, value) {
    if (typeof value !== "string")
        return value;
    if (key === "refreshToken" || key === "x-api-key") {
        return "REDACTED";
    }
    if (key === "accessToken") {
        return "REDACTED";
    }
    if (JWT_PATTERN.test(value)) {
        return `${value.slice(0, 8)}...REDACTED`;
    }
    return value;
}
export function redact(data) {
    const result = {};
    for (const [key, value] of Object.entries(data)) {
        result[key] = redactValue(key, value);
    }
    return result;
}
//# sourceMappingURL=logger.js.map