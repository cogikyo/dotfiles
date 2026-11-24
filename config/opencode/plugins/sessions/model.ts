import type { Plugin } from "@opencode/plugin";
import { Model } from "@opencode/schema/model";

type Listed = {
  readonly providerID: string;
  readonly id: string;
  readonly variants: readonly { readonly id: string }[];
};

export function missing(list: readonly Listed[], ref: { providerID: string; id: string; variant?: string }) {
  const found = list.filter((info) => info.providerID === ref.providerID && info.id === ref.id);
  if (!found.length) return `${ref.providerID}/${ref.id} is not an available model`;
  const variants = [...new Set(found.flatMap((info) => info.variants.map((option) => option.id)))];
  if (ref.variant && !variants.includes(ref.variant))
    return `${ref.providerID}/${ref.id} has no ${ref.variant} variant; it has ${variants.join(", ") || "none"}`;
  return undefined;
}

export async function available(ctx: Plugin.Context, tool: string, text: string) {
  const ref = Model.Ref.parse(text);
  const why = missing((await ctx.model.list()).data, ref);
  if (why) throw new Error(`${tool}: ${why}`);
  return ref;
}
