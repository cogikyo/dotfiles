import { RGBA } from "@opentui/core";
import type { Compiler } from "../vagari/build.ts";

export type Paint = { readonly fg: RGBA; readonly bold: boolean };

export function paint(compiler: Compiler, value: unknown, where: string): Paint | undefined {
  const [name, ...styles] = typeof value === "string" ? value.trim().split(/\s+/) : [value];
  const hex = compiler.hex(name, where);
  const unknown = styles.filter((style) => style !== "bold");
  if (unknown.length > 0) compiler.fail(where, value, `has unknown style ${unknown.join(", ")}; use bold`);
  return hex && unknown.length === 0 ? { fg: RGBA.fromHex(hex), bold: styles.length > 0 } : undefined;
}
