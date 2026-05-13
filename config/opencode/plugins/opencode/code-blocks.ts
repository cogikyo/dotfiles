// @ts-nocheck -- Patches OpenTUI internals that are not exposed through OpenCode's public TUI API.
import type { TuiPlugin, TuiPluginModule } from "@opencode-ai/plugin/tui";
import { MarkdownRenderable, RGBA, SyntaxStyle } from "@opentui/core";

const id = "opencode-code-blocks";
const PATCHED = Symbol.for("cullyn.opencode.code-blocks.patched");
const RENDER_PATCHED = Symbol.for("cullyn.opencode.code-blocks.render-patched");

const c = (hex: string) => RGBA.fromHex(hex);
const p = {
  fg: c("#aeb9f8"),
  brt_0: c("#b6c0f7"),
  brt_1: c("#bec6f8"),
  brt_3: c("#d1d8ff"),
  blu_1: c("#6380ec"),
  blu_2: c("#7492ef"),
  blu_3: c("#8aa4f3"),
  orn_3: c("#f2a170"),
  orn_4: c("#f8b486"),
  grn_1: c("#73ad5a"),
  grn_3: c("#95cb79"),
  grn_4: c("#9fd883"),
  prp_1: c("#9376d8"),
  prp_2: c("#a188df"),
  prp_3: c("#b29ae8"),
  rby_2: c("#f07a88"),
  rby_3: c("#f08898"),
  sun_2: c("#f5c069"),
  sun_4: c("#f5d599"),
  sky_0: c("#369fd7"),
  sky_1: c("#54b0e2"),
  sky_2: c("#6bbdec"),
  sky_4: c("#90d1f5"),
  cyn_2: c("#38d2ba"),
  pnk_2: c("#ea76c0"),
  pnk_4: c("#ed9acd"),
  glu_2: c("#7690b9"),
  glu_3: c("#90a4c7"),
  tyr_2: c("#5a9c78"),
  tyr_3: c("#72b08e"),
  slt_2: c("#484e75"),
};

const tui: TuiPlugin = async (api) => {
  const proto = MarkdownRenderable?.prototype as any;
  if (!proto || proto[PATCHED]) return;

  const originalCreateCodeRenderable = proto.createCodeRenderable;
  const originalApplyCodeBlockRenderable = proto.applyCodeBlockRenderable;
  if (typeof originalCreateCodeRenderable !== "function" || typeof originalApplyCodeBlockRenderable !== "function") {
    api.ui.toast({
      variant: "warning",
      title: "Code block styling disabled",
      message: "OpenTUI Markdown internals changed; opencode-code-blocks could not patch them.",
    });
    return;
  }

  const codeBlockBackground = () => api.theme.current.backgroundPanel;
  const syntaxStyle = SyntaxStyle.fromStyles({
    default: { fg: p.fg },
    comment: { fg: p.slt_2 },
    "comment.documentation": { fg: p.slt_2 },
    string: { fg: p.grn_3 },
    "string.documentation": { fg: p.glu_3 },
    "string.regexp": { fg: p.grn_1 },
    "string.escape": { fg: p.tyr_2 },
    "string.special": { fg: p.grn_4 },
    "string.special.path": { fg: p.grn_4 },
    "string.special.symbol": { fg: p.sky_1 },
    "string.special.url": { fg: p.tyr_3 },
    character: { fg: p.tyr_2 },
    "character.special": { fg: p.sky_0 },
    number: { fg: p.pnk_2 },
    "number.float": { fg: p.pnk_4, italic: true },
    boolean: { fg: p.cyn_2 },
    variable: { fg: p.fg },
    "variable.parameter": { fg: p.orn_4 },
    "variable.parameter.builtin": { fg: p.orn_3 },
    "variable.builtin": { fg: p.rby_3, italic: true },
    "variable.member": { fg: p.blu_3 },
    property: { fg: p.blu_3 },
    module: { fg: p.brt_3 },
    "module.go": { fg: p.tyr_3 },
    "module.builtin": { fg: p.blu_2, bold: true },
    label: { fg: p.prp_3 },
    function: { fg: p.blu_2, bold: true },
    "function.call": { fg: p.orn_4 },
    "function.method": { fg: p.blu_3 },
    "function.method.call": { fg: p.orn_3 },
    "function.builtin": { fg: p.blu_1, italic: true },
    "function.macro": { fg: p.blu_2, italic: true },
    constructor: { fg: p.brt_0, bold: true },
    type: { fg: p.brt_1 },
    "type.builtin": { fg: p.rby_3, italic: true },
    "type.definition": { fg: p.brt_3, bold: true },
    "type.qualifier": { fg: p.glu_2, italic: true },
    "type.parameter": { fg: p.glu_3, italic: true },
    attribute: { fg: p.prp_3, italic: true },
    "attribute.builtin": { fg: p.rby_3, italic: true },
    constant: { fg: p.sun_4 },
    "constant.builtin": { fg: p.rby_2, italic: true },
    "constant.macro": { fg: p.sun_2, italic: true },
    keyword: { fg: p.prp_2 },
    "keyword.function": { fg: p.prp_2, bold: true },
    "keyword.type": { fg: p.prp_2, bold: true },
    "keyword.return": { fg: p.prp_2, italic: true },
    "keyword.conditional": { fg: p.prp_1, italic: true },
    "keyword.repeat": { fg: p.prp_1, italic: true },
    "keyword.operator": { fg: p.prp_1, italic: true },
    "keyword.coroutine": { fg: p.prp_2, bold: true },
    "keyword.import": { fg: p.tyr_3, italic: true },
    "keyword.directive": { fg: p.tyr_3, italic: true },
    "keyword.directive.define": { fg: p.tyr_3, italic: true, bold: true },
    operator: { fg: p.sky_4, bold: true },
    "punctuation.bracket": { fg: p.glu_3 },
    "punctuation.delimiter": { fg: p.glu_2 },
    "punctuation.special": { fg: p.sky_2 },
    tag: { fg: p.blu_2 },
    "tag.builtin": { fg: p.blu_1, italic: true },
    "tag.attribute": { fg: p.orn_4, italic: true },
    "tag.delimiter": { fg: p.glu_2 },
  });

  const styleCodeBlock = (renderable: any) => {
    renderable.bg = codeBlockBackground();
    renderable.marginTop = 1;
    renderable.marginBottom = Math.max(Number(renderable.marginBottom ?? 0), 1);
    renderable.paddingTop = 1;
    renderable.paddingBottom = 0;
    renderable.paddingLeft = 2;
    renderable.paddingRight = 2;
    renderable.syntaxStyle = syntaxStyle;
    renderable.content = String(renderable.content ?? "").replace(/\n+$/g, "");

    if (!renderable[RENDER_PATCHED] && typeof renderable.renderSelf === "function") {
      const originalRenderSelf = renderable.renderSelf;
      renderable.renderSelf = function patchedRenderSelf(buffer: any, deltaTime: number) {
        buffer.fillRect(this.screenX, this.screenY, this.width, this.height, codeBlockBackground());
        const result = originalRenderSelf.call(this, buffer, deltaTime);
        drawFiletypeBadge(buffer, this);
        return result;
      };
      renderable[RENDER_PATCHED] = true;
    }
  };

  const drawFiletypeBadge = (buffer: any, renderable: any) => {
    const label = String(renderable.filetype ?? "").trim();
    if (!label || renderable.width < label.length + 4 || renderable.height < 1) return;

    const x = renderable.screenX + renderable.width - label.length - 2;
    const y = renderable.screenY + renderable.height - 1;
    buffer.drawText(label, x, y, api.theme.current.textMuted, codeBlockBackground());
  };

  proto.createCodeRenderable = function patchedCreateCodeRenderable(token: unknown, blockID: string, marginBottom = 0) {
    const renderable = originalCreateCodeRenderable.call(this, token, blockID, Math.max(Number(marginBottom ?? 0), 1));
    styleCodeBlock(renderable);
    return renderable;
  };

  proto.applyCodeBlockRenderable = function patchedApplyCodeBlockRenderable(renderable: unknown, token: unknown, marginBottom = 0) {
    originalApplyCodeBlockRenderable.call(this, renderable, token, Math.max(Number(marginBottom ?? 0), 1));
    styleCodeBlock(renderable);
  };

  proto[PATCHED] = true;
};

const plugin: TuiPluginModule & { id: string } = {
  id,
  tui,
};

export default plugin;
