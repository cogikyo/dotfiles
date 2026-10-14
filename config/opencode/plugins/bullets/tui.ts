import { Plugin } from "@opencode/plugin/tui";
import { MarkdownRenderable } from "@opentui/core";

type Inputs = (token: { ordered?: boolean }, id: string) => { marker: string }[];

const markers = ["•", "◦"];

export default Plugin.define({
  id: "bullets",
  setup() {
    // V2 exposes only code-block renderers, so list markers need this private opentui override.
    const proto = MarkdownRenderable.prototype as unknown as { getListItemInputs: Inputs };
    const native = proto.getListItemInputs;
    if (typeof native !== "function") throw new Error("MarkdownRenderable.getListItemInputs no longer exists");
    proto.getListItemInputs = function (token, id) {
      const inputs = native.call(this, token, id);
      if (token.ordered) return inputs;
      // Nested list ids inherit one "-item-" segment per enclosing list item.
      const depth = id.split("-item-").length - 1;
      const marker = markers[Math.min(depth, markers.length - 1)];
      for (const input of inputs) input.marker = marker;
      return inputs;
    };
    return () => {
      proto.getListItemInputs = native;
    };
  },
});
