import { Plugin } from "@opencode/plugin/tui";
import { addDefaultParsers, getTreeSitterClient, infoStringToFiletype } from "@opentui/core";
import { card } from "./card.ts";
import { scan } from "./parsers.ts";
import { load } from "./vagari.ts";

const plain = ["console", "text", "txt", "plaintext"];

function fence(name: string): string {
  return infoStringToFiletype(name) ?? name;
}

export default Plugin.define({
  id: "code-blocks",
  setup(context) {
    const languages = scan();
    const vagari = load();
    const parsers = languages.flatMap((language) => (language.parser ? [language.parser] : []));
    addDefaultParsers(parsers);
    const client = getTreeSitterClient();
    void client.initialize().then(
      () => parsers.forEach((parser) => client.addFiletypeParser(parser)),
      (error: unknown) =>
        context.ui.toast.show({
          variant: "error",
          title: "Code block parsers",
          message: error instanceof Error ? error.message : String(error),
        }),
    );

    const renderers = new Map(plain.map((name) => [fence(name), card(context, vagari, undefined, vagari.style(""))]));
    for (const language of languages) {
      const render = card(context, vagari, language.filetype, vagari.style(language.name));
      for (const name of [language.name, ...language.aliases]) renderers.set(fence(name), render);
    }
    for (const [name, render] of renderers) context.markdown.registerCodeBlockRenderer(name, render);
    return () => vagari.destroy();
  },
});
