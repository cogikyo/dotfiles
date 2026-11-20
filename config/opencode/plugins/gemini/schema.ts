import { Schema } from "effect";

type Node = Record<string, unknown>;

const types = new Set(["string", "number", "integer", "boolean", "array", "object"]);
const kept = [
  "title",
  "description",
  "pattern",
  "minLength",
  "maxLength",
  "minItems",
  "maxItems",
  "minimum",
  "maximum",
  "minProperties",
  "maxProperties",
];
const formats: Record<string, string[]> = {
  string: ["enum", "date-time"],
  number: ["float", "double"],
  integer: ["int32", "int64"],
};

export function gemini(input: unknown): Node | undefined {
  try {
    const root = shape(inline(flatten(json(input))));
    return root.type === "object" ? root : undefined;
  } catch {
    return undefined;
  }
}

function json(input: unknown): unknown {
  if (!Schema.isSchema(input)) return input ?? { type: "object", properties: {} };
  const document = Schema.toJsonSchemaDocument(input, { onExcessProperty: "error" });
  const { not, ...rest } = document.schema;
  if (rest.type === undefined && not !== undefined) return { ...rest, type: "object", properties: {} };
  return Object.keys(document.definitions).length === 0
    ? document.schema
    : { ...document.schema, $defs: document.definitions };
}

function flatten(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(flatten);
  if (!record(value)) return value;
  const integer = value.type === "integer";
  const node = Object.fromEntries(
    Object.entries(value)
      .filter(([key, item]) => !integer || !(key === "minimum" || key === "maximum") || !safe(item))
      .map(([key, item]) => [key, flatten(item)]),
  );
  const { allOf, ...rest } = node;
  if (!Array.isArray(allOf) || !allOf.every(record)) return node;
  const keys = [...Object.keys(rest), ...allOf.flatMap((item) => Object.keys(item))];
  if (new Set(keys).size < keys.length) return node;
  return flatten(Object.assign({}, ...allOf, rest));
}

function inline(value: unknown, definitions?: unknown, seen = new Set<string>()): unknown {
  if (Array.isArray(value)) return value.map((item) => inline(item, definitions, seen));
  if (!record(value)) return value;
  const { $defs, $ref, ...rest } = value;
  const local = definitions ?? $defs;
  const children = Object.fromEntries(Object.entries(rest).map(([key, item]) => [key, inline(item, local, seen)]));
  if ($ref === undefined) return children;
  const name = typeof $ref === "string" ? /^#\/\$defs\/([^/]+)$/.exec($ref)?.[1] : undefined;
  const target = name === undefined || !record(local) ? undefined : local[name];
  if (name === undefined || seen.has(name) || !record(target)) throw new Error(`cannot inline ${JSON.stringify($ref)}`);
  const resolved = inline(target, local, new Set(seen).add(name));
  return { ...(record(resolved) ? resolved : {}), ...children };
}

function shape(value: unknown): Node {
  if (value === true) return {};
  if (!record(value)) throw new Error("schema is not an object");
  if (value.allOf !== undefined) throw new Error("allOf could not be flattened");
  const node: Node = Object.fromEntries(kept.flatMap((key) => (value[key] === undefined ? [] : [[key, value[key]]])));
  if (typeof value.exclusiveMinimum === "number") node.minimum = value.exclusiveMinimum;
  if (typeof value.exclusiveMaximum === "number") node.maximum = value.exclusiveMaximum;
  const kinds = values(value, node);
  const choice = choose(value, node);
  if (choice) return { ...choice, ...node };
  const kind = typed(value, kinds, node);
  if (kind === "object") node.properties = properties(value, node);
  if (kind === "array") node.items = shape(items(value));
  return node;
}

function values(value: Node, node: Node) {
  const listed = value.const !== undefined ? [value.const] : Array.isArray(value.enum) ? value.enum : [];
  const declared: unknown[] = Array.isArray(value.type) ? value.type : value.type === undefined ? [] : [value.type];
  if (listed.includes(null) || declared.includes("null")) node.nullable = true;
  const kinds = declared.filter((item) => item !== "null");
  const options = listed.filter((item) => item !== null);
  if (options.length === 0) return kinds;
  if (options.every((item) => typeof item === "string" && item !== "")) {
    node.enum = [...new Set(options)];
    return ["string"];
  }
  const found = [...new Set(options.map((item) => typeof item))];
  if (found.length !== 1 || (found[0] !== "boolean" && found[0] !== "number")) throw new Error("enum mixes kinds");
  const allowed = `Allowed: ${options.map((item) => JSON.stringify(item)).join(", ")}.`;
  node.description = typeof node.description === "string" ? `${node.description} ${allowed}` : allowed;
  return kinds.length === 0 ? found : kinds;
}

function choose(value: Node, node: Node) {
  const choices = Array.isArray(value.anyOf) ? value.anyOf : Array.isArray(value.oneOf) ? value.oneOf : undefined;
  if (!choices) return undefined;
  const options = choices.filter((item) => !(record(item) && item.type === "null" && Object.keys(item).length === 1));
  if (options.length < choices.length) node.nullable = true;
  const shaped = options.map(shape);
  if (shaped.length === 1) return shaped[0];
  if (shaped.length === 0) throw new Error("no non-null choice");
  node.anyOf = shaped;
  return {};
}

function typed(value: Node, kinds: unknown[], node: Node) {
  const inferred =
    kinds.length > 0
      ? kinds
      : record(value.properties)
        ? ["object"]
        : value.items !== undefined || value.prefixItems !== undefined
          ? ["array"]
          : [];
  const known = inferred.filter((kind): kind is string => typeof kind === "string" && types.has(kind));
  if (known.length < inferred.length) throw new Error("unsupported type");
  if (known.length > 1) {
    node.anyOf = known.map((kind) => ({ type: kind }));
    return undefined;
  }
  const kind = known[0];
  if (kind === undefined) return undefined;
  node.type = kind;
  if (typeof value.format === "string" && formats[kind]?.includes(value.format)) node.format = value.format;
  return kind;
}

function properties(value: Node, node: Node) {
  const declared = record(value.properties) ? value.properties : {};
  const mapped = Object.fromEntries(Object.entries(declared).map(([name, item]) => [name, shape(item)]));
  const required = Array.isArray(value.required) ? value.required.filter((name) => Object.hasOwn(mapped, name)) : [];
  if (required.length > 0) node.required = required;
  return mapped;
}

function items(value: Node) {
  if (Array.isArray(value.prefixItems)) return { anyOf: value.prefixItems };
  return value.items === undefined ? true : value.items;
}

function safe(bound: unknown) {
  return bound === Number.MIN_SAFE_INTEGER || bound === Number.MAX_SAFE_INTEGER;
}

function record(value: unknown): value is Node {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
