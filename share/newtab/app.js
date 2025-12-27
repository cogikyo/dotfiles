// ═══════════════════════════════════════════════════════════════════════════
// Constants
// ═══════════════════════════════════════════════════════════════════════════

const TOP_LEFT_FOLDERS = ["localhost", "trend", "git"];
const TOP_RIGHT_FOLDERS = ["google", "x"];
const DEBOUNCE_MS = 10;
const AUTO_GO_MS = 200;

// ═══════════════════════════════════════════════════════════════════════════
// State
// ═══════════════════════════════════════════════════════════════════════════

let bookmarks = [];
let folders = [];
let activeFolder = null;
let selected = -1;
let autoGoTimer = null;
let suggestTimer = null;
let historyResults = [];
let suggestions = [];

// ═══════════════════════════════════════════════════════════════════════════
// DOM
// ═══════════════════════════════════════════════════════════════════════════

const topLeftBar = document.querySelector(".toolbar.top-left");
const topRightBar = document.querySelector(".toolbar.top-right");
const bottomBar = document.querySelector(".toolbar.bottom");
const input = document.querySelector("input");
const suggestionsList = document.querySelector(".suggestions");
const loadingEl = document.querySelector(".loading");
const loadingText = document.querySelector(".loading-text");

// ═══════════════════════════════════════════════════════════════════════════
// API
// ═══════════════════════════════════════════════════════════════════════════

async function fetchBookmarks() {
  try {
    const res = await fetch("/api/bookmarks");
    return (await res.json()) || [];
  } catch {
    console.warn("Failed to fetch bookmarks");
    return [];
  }
}

async function fetchHistory(query) {
  try {
    const url = query ? `/api/history?q=${encodeURIComponent(query)}` : "/api/history";
    const res = await fetch(url);
    const data = await res.json();
    return (data || []).map((h) => ({
      title: h.title,
      url: h.url,
      visitCount: h.visit_count,
      type: "history",
    }));
  } catch {
    return [];
  }
}

async function fetchSuggestions(query) {
  if (!query) return [];
  try {
    const res = await fetch(`/api/suggest?q=${encodeURIComponent(query)}`);
    const data = await res.json();
    return (data || []).map((s) => ({
      title: s,
      type: "suggested",
    }));
  } catch {
    return [];
  }
}

// ═══════════════════════════════════════════════════════════════════════════
// Utils
// ═══════════════════════════════════════════════════════════════════════════

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

// ═══════════════════════════════════════════════════════════════════════════
// Search
// ═══════════════════════════════════════════════════════════════════════════

function searchBookmarks(query) {
  const pool = activeFolder ? bookmarks.filter((b) => b.folder === activeFolder) : bookmarks;

  if (!query) return activeFolder ? pool.slice(0, 10).map((b) => ({ ...b, type: "bookmark" })) : [];

  for (const b of pool) {
    if (b.keyword && b.keyword === query) {
      return [{ ...b, exact: true, type: "bookmark" }];
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
      results.push({ ...b, score: best.score, field: best.field, pos: best.pos, type: "bookmark" });
    }
  }

  results.sort((a, b) => b.score - a.score);
  return results.slice(0, 8);
}

function blendResults(bookmarkResults, historyResults, suggestionResults, query) {
  const results = [];
  const seenUrls = new Set();

  for (const b of bookmarkResults) {
    if (!seenUrls.has(b.url)) {
      seenUrls.add(b.url);
      results.push(b);
    }
  }

  for (const s of suggestionResults.slice(0, 5)) {
    const match = query ? fuzzy(s.title, query) : null;
    results.push({ ...s, pos: match?.pos });
  }

  if (!activeFolder) {
    let historyCount = 0;
    for (const h of historyResults) {
      if (historyCount >= 3) break;
      if (!seenUrls.has(h.url)) {
        seenUrls.add(h.url);
        const match = query ? fuzzy(h.title, query) : null;
        results.push({ ...h, pos: match?.pos });
        historyCount++;
      }
    }
  }

  return results.slice(0, 12);
}

// ═══════════════════════════════════════════════════════════════════════════
// Render
// ═══════════════════════════════════════════════════════════════════════════

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

function updateSelection() {
  suggestionsList.querySelectorAll(".suggestion").forEach((el) => {
    el.classList.toggle("selected", +el.dataset.idx === selected);
  });
}

function render(results, query) {
  const exactKeyword = results.length > 0 && results[0].exact;
  const exactName =
    results.length > 0 &&
    results[0].type === "bookmark" &&
    clean(results[0].title).toLowerCase() === query.toLowerCase();
  const exactMatch = exactKeyword || exactName;

  input.classList.toggle("keyword-match", exactMatch);

  if (exactMatch && results[0].folder) {
    input.dataset.folder = results[0].folder;
  } else {
    delete input.dataset.folder;
  }

  if (!query && !activeFolder) {
    suggestionsList.classList.remove("active");
    suggestionsList.innerHTML = "";
    return;
  }

  let html = "";
  let lastType = null;

  results.forEach((r, i) => {
    if (lastType === "suggested" && r.type === "history") {
      html += `<div class="suggestion-divider"></div>`;
    }
    lastType = r.type;

    const title = clean(r.title || "");
    const display = r.pos ? highlight(title, r.pos) : title;
    const typeClass =
      r.type === "bookmark" ? (r.keyword ? "type-bookmark-keyword" : "type-bookmark") : r.type ? `type-${r.type}` : "";
    const cls = ["suggestion", i === selected ? "selected" : "", r.exact ? "keyword-exact" : "", typeClass]
      .filter(Boolean)
      .join(" ");

    const meta = r.folder
      ? `<span class="folder">${r.folder}</span>`
      : r.type === "suggested"
      ? `<span class="folder">suggested</span>`
      : r.type === "history"
      ? `<span class="folder">history</span>`
      : `<span class="folder type-indicator">★</span>`;

    html += `
      <div class="${cls}" data-idx="${i}" data-folder="${r.folder || ""}" style="--i:${i}">
        <span class="title">${r.keyword ? `<span class="keyword">${r.keyword}</span>` : ""}${display}</span>
        <span class="meta">${meta}</span>
      </div>`;
  });

  if (query) {
    const fallbackIdx = results.length;
    const fallbackSelected = selected === fallbackIdx;
    html += `
      <div class="suggestion fallback ${fallbackSelected ? "selected" : ""}" data-idx="${fallbackIdx}">
        <span class="title">${query}</span>
        <span class="meta"><span class="folder">google</span></span>
      </div>`;
  }

  suggestionsList.innerHTML = html;
  suggestionsList.classList.add("active");

  suggestionsList.querySelectorAll(".suggestion").forEach((el) => {
    el.addEventListener("mousedown", (e) => {
      e.preventDefault();
      go(+el.dataset.idx, results, query);
    });
  });
}

// ═══════════════════════════════════════════════════════════════════════════
// Navigation
// ═══════════════════════════════════════════════════════════════════════════

function toggleFolder(folder) {
  activeFolder = activeFolder === folder ? null : folder;
  updateToolbarState();
  input.focus();
  update();
}

function truncateUrl(url, max = 50) {
  if (url.length <= max) return url;
  return url.slice(0, max - 3) + "...";
}

function showLoading(label, url, folder) {
  input.style.display = "none";
  suggestionsList.style.display = "none";
  loadingEl.classList.remove("hidden");
  if (folder) {
    loadingEl.dataset.folder = folder;
    delete loadingEl.dataset.type;
  } else {
    loadingEl.dataset.type = label;
    delete loadingEl.dataset.folder;
  }
  loadingText.innerHTML = `<span class="folder">${label}</span>${truncateUrl(url)}`;
}

function go(idx, results, query) {
  let url, label, folder;
  if (idx < results.length) {
    const r = results[idx];
    if (r.type === "suggested") {
      url = `https://google.com/search?q=${encodeURIComponent(r.title)}`;
      label = "suggested";
    } else if (r.type === "history") {
      url = r.url;
      label = "history";
    } else {
      url = r.url;
      label = r.folder || "bookmark";
      folder = r.folder;
    }
  } else {
    url = `https://google.com/search?q=${encodeURIComponent(query)}`;
    label = "google";
  }
  showLoading(label, url, folder);
  requestAnimationFrame(() => {
    location.href = url;
  });
}

function reset() {
  clearTimeout(autoGoTimer);
  clearTimeout(suggestTimer);
  input.value = "";
  activeFolder = null;
  historyResults = [];
  suggestions = [];
  updateToolbarState();
  suggestionsList.classList.remove("active");
  input.classList.remove("keyword-match");
  delete input.dataset.folder;
  selected = -1;
}

// ═══════════════════════════════════════════════════════════════════════════
// Update Loop
// ═══════════════════════════════════════════════════════════════════════════

async function update() {
  clearTimeout(autoGoTimer);
  clearTimeout(suggestTimer);

  const q = input.value.trim();
  const bookmarkResults = searchBookmarks(q);
  const hasExactMatch = bookmarkResults.length === 1 && bookmarkResults[0].exact;

  if (hasExactMatch) {
    selected = 0;
    render(bookmarkResults, q);
    const b = bookmarkResults[0];
    autoGoTimer = setTimeout(() => {
      showLoading(b.folder || "bookmark", b.url, b.folder);
      requestAnimationFrame(() => {
        location.href = b.url;
      });
    }, AUTO_GO_MS);
    return;
  }

  const results = blendResults(bookmarkResults, historyResults, suggestions, q);
  const noBookmarks = bookmarkResults.length === 0;
  selected = q ? (noBookmarks ? results.length : 0) : results.length > 0 ? 0 : -1;
  render(results, q);

  suggestTimer = setTimeout(async () => {
    const [newHistory, newSuggestions] = await Promise.all([fetchHistory(q), fetchSuggestions(q)]);

    historyResults = newHistory;
    suggestions = newSuggestions;

    const currentQ = input.value.trim();
    const currentBookmarks = searchBookmarks(currentQ);
    if (currentBookmarks.length === 1 && currentBookmarks[0].exact) {
      return;
    }

    const results = blendResults(currentBookmarks, historyResults, suggestions, currentQ);
    const noBookmarks = currentBookmarks.length === 0;
    selected = currentQ ? (noBookmarks ? results.length : 0) : results.length > 0 ? 0 : -1;
    render(results, currentQ);
  }, DEBOUNCE_MS);
}

// ═══════════════════════════════════════════════════════════════════════════
// Events
// ═══════════════════════════════════════════════════════════════════════════

input.addEventListener("input", update);

input.addEventListener("blur", () => {
  if (activeFolder) {
    activeFolder = null;
    updateToolbarState();
    update();
  }
});

input.addEventListener("keydown", (e) => {
  const q = input.value.trim();
  const bookmarkResults = searchBookmarks(q);
  const results = blendResults(bookmarkResults, historyResults, suggestions, q);
  const total = results.length + (q ? 1 : 0);

  switch (e.key) {
    case "ArrowDown":
      e.preventDefault();
      selected = (selected + 1) % total;
      updateSelection();
      break;

    case "ArrowUp":
      e.preventDefault();
      selected = selected <= 0 ? total - 1 : selected - 1;
      updateSelection();
      break;

    case "Tab":
      e.preventDefault();
      const dir = e.shiftKey ? -1 : 1;
      const idx = folders.indexOf(activeFolder);
      const next = (idx + dir + folders.length + 1) % (folders.length + 1);
      activeFolder = next < folders.length ? folders[next] : null;
      updateToolbarState();
      update();
      break;

    case "Enter":
      e.preventDefault();
      if (bookmarkResults.length === 1 && bookmarkResults[0].exact) {
        const b = bookmarkResults[0];
        showLoading(b.folder || "bookmark", b.url, b.folder);
        requestAnimationFrame(() => {
          location.href = b.url;
        });
      } else if (selected >= 0) {
        go(selected, results, q);
      } else if (results.length > 0) {
        go(0, results, q);
      } else if (q) {
        const url = `https://google.com/search?q=${encodeURIComponent(q)}`;
        showLoading("google", url);
        requestAnimationFrame(() => {
          location.href = url;
        });
      }
      break;

    case "Escape":
      reset();
      break;
  }
});

// ═══════════════════════════════════════════════════════════════════════════
// Init
// ═══════════════════════════════════════════════════════════════════════════

function resetLoading() {
  loadingEl.classList.add("hidden");
  delete loadingEl.dataset.type;
  delete loadingEl.dataset.folder;
  input.style.display = "";
  suggestionsList.style.display = "";
}

async function init() {
  resetLoading();
  input.value = "";
  bookmarks = await fetchBookmarks();
  folders = [...new Set(bookmarks.map((b) => b.folder))].sort();
  renderToolbar();
}

window.addEventListener("pageshow", (e) => {
  if (e.persisted) {
    resetLoading();
    reset();
  }
});

init();
