import path from "node:path";

export const separator = "\u0000";

export function commandArgs(words: string[], index: number) {
  const end = words.indexOf(separator, index + 1);
  return words.slice(index + 1, end === -1 ? undefined : end);
}

export function nestedShellCommands(words: string[]) {
  return words.filter((_, index) => {
    if (index < 2 || !/^-\w*c\w*$/u.test(words[index - 1])) return false;
    return ["bash", "dash", "sh", "zsh"].includes(executable(words[index - 2]));
  });
}

export function shellWords(command: string) {
  const words: string[] = [];
  let word = "";
  let quote = "";
  let escaped = false;

  const flush = () => {
    if (word) words.push(word);
    word = "";
  };

  for (const char of command) {
    if (escaped) {
      word += char;
      escaped = false;
      continue;
    }
    if (char === "\\" && quote !== "'") {
      escaped = true;
      continue;
    }
    if (quote) {
      if (char === quote) quote = "";
      else word += char;
      continue;
    }
    if (char === "'" || char === '"') {
      quote = char;
      continue;
    }
    if (/[\n;&|()]/u.test(char)) {
      flush();
      if (words.at(-1) !== separator) words.push(separator);
      continue;
    }
    if (/\s/u.test(char)) {
      flush();
      continue;
    }
    word += char;
  }
  flush();
  return words;
}

export function executable(word: string) {
  return path.basename(word);
}
