let bookmarks = [];
let folders = [];
let activeFolder = null;
let selected = -1;
let autoGoTimer = null;

const TOP_LEFT_FOLDERS = ["localhost", "trend", "git"];
const TOP_RIGHT_FOLDERS = ["google", "x"];

const topLeftBar = document.querySelector(".toolbar.top-left");
const topRightBar = document.querySelector(".toolbar.top-right");
const bottomBar = document.querySelector(".toolbar.bottom");
const input = document.querySelector("input");
const suggestions = document.querySelector(".suggestions");

input.value = "";

fetch("bookmarks.json")
  .then((r) => r.json())
  .then((data) => {
    bookmarks = data;
    folders = [...new Set(data.map((b) => b.folder))].sort();
    renderToolbar();
  })
  .catch(() => console.warn("bookmarks.json not found"));

function renderToolbar() {
  const topLeft = TOP_LEFT_FOLDERS.filter((f) => folders.includes(f));
  const topRight = TOP_RIGHT_FOLDERS.filter((f) => folders.includes(f));
  const bottom = folders.filter((f) => !TOP_LEFT_FOLDERS.includes(f) && !TOP_RIGHT_FOLDERS.includes(f));

  topLeftBar.innerHTML = topLeft.map((f, i) => `<button data-folder="${f}" style="--i:${i}">${f}</button>`).join("");

  topRightBar.innerHTML = topRight.map((f, i) => `<button data-folder="${f}" style="--i:${i}">${f}</button>`).join("");

  bottomBar.innerHTML = bottom.map((f, i) => `<button data-folder="${f}" style="--i:${i}">${f}</button>`).join("");

  document.querySelectorAll(".toolbar button").forEach((btn) => {
    btn.addEventListener("click", () => toggleFolder(btn.dataset.folder));
  });
}

function toggleFolder(folder) {
  activeFolder = activeFolder === folder ? null : folder;
  updateToolbarState();
  input.focus();
  update();
}

function updateToolbarState() {
  document.querySelectorAll(".toolbar button").forEach((btn) => {
    btn.classList.toggle("active", btn.dataset.folder === activeFolder);
  });

  if (activeFolder) {
    document.body.dataset.folder = activeFolder;
  } else {
    delete document.body.dataset.folder;
  }
}

function clean(title) {
  return title.replace(/\s*[\(\[@][\w@]+[\)\]@]?\s*$/, "").trim();
}

function fuzzy(text, query) {
  const t = text.toLowerCase();
  const q = query.toLowerCase();
  let score = 0,
    ti = 0,
    qi = 0,
    run = 0,
    pos = [];

  while (ti < t.length && qi < q.length) {
    if (t[ti] === q[qi]) {
      pos.push(ti);
      run++;
      score += 1 + run;
      if (ti === 0 || /\W/.test(t[ti - 1])) score += 5;
      qi++;
    } else {
      run = 0;
    }
    ti++;
  }

  if (qi < q.length) return null;
  if (t.startsWith(q)) score += 15;
  if (t === q) score += 50;

  return { score, pos };
}

function search(query) {
  const pool = activeFolder ? bookmarks.filter((b) => b.folder === activeFolder) : bookmarks;

  if (!query) return activeFolder ? pool.slice(0, 10) : [];

  for (const b of pool) {
    if (b.keyword && b.keyword === query) {
      return [{ ...b, exact: true }];
    }
  }

  const results = [];

  for (const b of pool) {
    const title = clean(b.title);
    const kw = b.keyword || "";
    let best = null;

    if (kw) {
      const kwMatch = fuzzy(kw, query);
      if (kwMatch) best = { ...kwMatch, field: "keyword" };
    }

    const titleMatch = fuzzy(title, query);
    if (titleMatch && (!best || titleMatch.score > best.score)) {
      best = { ...titleMatch, field: "title" };
    }

    if (!activeFolder) {
      const folderMatch = fuzzy(b.folder, query);
      if (folderMatch && folderMatch.score > 10) {
        if (!best || folderMatch.score > best.score - 5) {
          if (!best) best = { ...folderMatch, field: "folder" };
        }
      }
    }

    if (best) {
      results.push({ ...b, score: best.score, field: best.field, pos: best.pos });
    }
  }

  results.sort((a, b) => b.score - a.score);
  return results.slice(0, 10);
}

function highlight(text, pos) {
  if (!pos?.length) return text;
  let out = "",
    last = 0;
  for (const p of pos) {
    out += text.slice(last, p) + `<span class="match">${text[p]}</span>`;
    last = p + 1;
  }
  return out + text.slice(last);
}

function updateSelection() {
  suggestions.querySelectorAll(".suggestion").forEach((el) => {
    el.classList.toggle("selected", +el.dataset.idx === selected);
  });
}

function render(results, query) {
  const exactKeyword = results.length === 1 && results[0].exact;
  const exactName = results.length > 0 && clean(results[0].title).toLowerCase() === query.toLowerCase();
  const exactMatch = exactKeyword || exactName;

  input.classList.toggle("keyword-match", exactMatch);

  if (exactMatch) {
    input.dataset.folder = results[0].folder;
  } else {
    delete input.dataset.folder;
  }

  if (!query && !activeFolder) {
    suggestions.classList.remove("active");
    suggestions.innerHTML = "";
    return;
  }

  let html = results
    .map((r, i) => {
      const title = clean(r.title);
      const display = r.field === "title" && r.pos ? highlight(title, r.pos) : title;
      const cls = ["suggestion", i === selected ? "selected" : "", r.exact ? "keyword-exact" : ""]
        .filter(Boolean)
        .join(" ");

      return `
        <div class="${cls}" data-idx="${i}" data-folder="${r.folder}" style="--i:${i}">
          <span class="title">${r.keyword ? `<span class="keyword">${r.keyword}</span>` : ""}${display}</span>
          <span class="meta">
            <span class="folder">${r.folder}</span>
          </span>
        </div>`;
    })
    .join("");

  if (query) {
    const fallbackSelected = results.length === 0 && selected === 0;
    html += `
      <div class="suggestion fallback ${fallbackSelected ? "selected" : ""}" data-idx="${results.length}">
        <span class="title">search: "${query}"</span>
        <span class="meta"><span class="folder">google</span></span>
      </div>`;
  }

  suggestions.innerHTML = html;
  suggestions.classList.add("active");

  suggestions.querySelectorAll(".suggestion").forEach((el) => {
    el.addEventListener("click", () => go(+el.dataset.idx, results, query));
  });
}

function go(idx, results, query) {
  if (idx < results.length) {
    location.href = results[idx].url;
  } else {
    location.href = `https://google.com/search?q=${encodeURIComponent(query)}`;
  }
}

function update() {
  clearTimeout(autoGoTimer);
  selected = -1;
  const q = input.value.trim();
  const results = search(q);

  if (results.length === 1 && results[0].exact) {
    selected = 0;
    autoGoTimer = setTimeout(() => {
      location.href = results[0].url;
    }, 200);
  }

  render(results, q);
}

input.addEventListener("input", update);

input.addEventListener("keydown", (e) => {
  const q = input.value.trim();
  const results = search(q);
  const total = results.length + (q ? 1 : 0);

  if (e.key === "ArrowDown") {
    e.preventDefault();
    selected = (selected + 1) % total;
    updateSelection();
  } else if (e.key === "ArrowUp") {
    e.preventDefault();
    selected = selected <= 0 ? total - 1 : selected - 1;
    updateSelection();
  } else if (e.key === "Tab") {
    e.preventDefault();
    const dir = e.shiftKey ? -1 : 1;
    const idx = folders.indexOf(activeFolder);
    const next = (idx + dir + folders.length + 1) % (folders.length + 1);
    activeFolder = next < folders.length ? folders[next] : null;
    updateToolbarState();
    update();
  } else if (e.key === "Enter") {
    e.preventDefault();
    if (results.length === 1 && results[0].exact) {
      location.href = results[0].url;
    } else if (selected >= 0) {
      go(selected, results, q);
    } else if (results.length > 0) {
      location.href = results[0].url;
    } else if (q) {
      location.href = `https://google.com/search?q=${encodeURIComponent(q)}`;
    }
  } else if (e.key === "Escape") {
    clearTimeout(autoGoTimer);
    input.value = "";
    activeFolder = null;
    updateToolbarState();
    suggestions.classList.remove("active");
    input.classList.remove("keyword-match");
    delete input.dataset.folder;
    selected = -1;
  }
});
