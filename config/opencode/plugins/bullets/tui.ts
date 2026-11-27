import { Plugin } from "@opencode/plugin/tui";
import { MarkdownRenderable, StyledText, type TextRenderable } from "@opentui/core";

const markers = ["•", "◦"];

type Host = {
  _doc: { blocks: { parent?: number }[] };
  _shown: ({ marker?: TextRenderable } | undefined)[];
  childrenOf(index: number): number[];
};
type Update = (this: Host, index: number, listed?: number[]) => void;

export default Plugin.define({
  id: "bullets",
  setup() {
    // OpenTUI 0.9.9 hard-codes "-" in its private updateList, so rewrite the markers after it.
    const proto = MarkdownRenderable.prototype as unknown as { updateList: Update };
    const native = proto.updateList;
    if (typeof native !== "function")
      throw new Error("MarkdownRenderable.updateList no longer exists; recheck the OpenTUI version");
    proto.updateList = function (index, listed) {
      native.call(this, index, listed);
      const blocks = this._doc.blocks;
      // Only list items have markers, so marked ancestors are the enclosing items.
      let depth = 0;
      for (let at = blocks[index]?.parent; at !== undefined; at = blocks[at]?.parent)
        if (this._shown[at]?.marker) depth++;
      const bullet = markers[Math.min(depth, markers.length - 1)];
      for (const item of this.childrenOf(index)) {
        const marker = this._shown[item]?.marker;
        const [chunk, ...rest] = marker?.chunks ?? [];
        // Skip ordered labels and markers native left alone, which already hold the bullet.
        if (!marker || !chunk || rest.length > 0 || !chunk.text.startsWith("-")) continue;
        marker.content = new StyledText([{ ...chunk, text: bullet + chunk.text.slice(1) }]);
      }
    };
    return () => {
      proto.updateList = native;
    };
  },
});
