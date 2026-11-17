import { engines, firefox, MCP, type Engine } from "./mcp.ts";
import * as Profile from "./profile.ts";

type Failure = (message: string, fields: { browser: string; error: unknown }) => void;

export class Browsers {
  private readonly open = new Map<string, Promise<MCP>>();
  private readonly closing = new Set<Promise<void>>();
  private readonly lanes = new WeakMap<MCP, Promise<unknown>>();

  constructor(private readonly fail: Failure) {}

  get(engine: Engine, sessionID: string) {
    const key = `${engine.name}/${sessionID}`;
    const existing = this.open.get(key);
    if (existing) return existing;
    const opening = engine === firefox ? this.launch(key, sessionID) : MCP.connect(engine);
    this.open.set(key, opening);
    opening.then(
      (browser) => browser.exited.then(() => this.forget(key, opening)),
      () => this.forget(key, opening),
    );
    return opening;
  }

  async call(engine: Engine, sessionID: string, name: string, input: unknown) {
    const browser = await this.get(engine, sessionID);
    const result = (this.lanes.get(browser) ?? Promise.resolve()).then(() => browser.call(name, input));
    this.lanes.set(
      browser,
      result.catch(() => undefined),
    );
    return result;
  }

  close(sessionID: string) {
    for (const engine of engines) this.shut(`${engine.name}/${sessionID}`);
  }

  async clear() {
    for (const key of this.open.keys()) this.shut(key);
    await this.drain();
  }

  private async drain(): Promise<void> {
    if (this.closing.size === 0) return;
    await Promise.all(this.closing);
    return this.drain();
  }

  private async launch(key: string, sessionID: string) {
    const profile = await Profile.copy(sessionID);
    if (!profile) return MCP.connect(firefox);
    const browser = await MCP.connect(firefox, [
      `--profile-path=${profile}`,
      "--disable-network-body-collection",
    ]).catch(async (error: unknown) => {
      await Profile.remove(profile);
      throw error;
    });
    this.track(
      key,
      "browser: profile removal failed",
      browser.exited.then(() => Profile.remove(profile)),
    );
    return browser;
  }

  private shut(key: string) {
    const browser = this.open.get(key);
    if (!browser) return;
    this.open.delete(key);
    this.track(
      key,
      "browser: close failed",
      browser.catch(() => undefined).then((mcp) => mcp?.close()),
    );
  }

  private forget(key: string, browser: Promise<MCP>) {
    if (this.open.get(key) === browser) this.open.delete(key);
  }

  private track(key: string, message: string, work: Promise<unknown>) {
    const done = work
      .then(
        () => undefined,
        (error: unknown) => this.fail(message, { browser: key, error }),
      )
      .finally(() => this.closing.delete(done));
    this.closing.add(done);
  }
}
