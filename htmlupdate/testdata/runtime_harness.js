// Harness driving the browser runtime under node.
//
// It stubs only what the runtime touches, and deliberately does not implement
// HTML parsing: a replacement is matched by its instance attribute and swapped
// in a registry. What this verifies is the protocol half of the runtime -
// header construction, version checking, validator bookkeeping, supersession,
// and fallback - which is the half a browser test would be a clumsy way to
// cover. Real DOM insertion is the browser's job.

const fs = require("fs");

const boundaries = new Map([
  ["c1", { id: "c1" }],
  ["c2", { id: "c2" }],
]);

// A boundary fragment carries the framework attribute; a redraw or action
// fragment carries the author's own id.
function idOf(html) {
  const match = /data-tb-id="([^"]+)"/.exec(html) || /\bid="([^"]+)"/.exec(html);
  return match ? match[1] : null;
}

const requests = [];
let response = null;
let assigned = null;

// Regions addressed by an author-written id, as a redraw or an action does.
const authorRegions = new Map([["cart", { id: "cart" }]]);

function swapper(registry, node) {
  node.replaceWith = (replacement) => {
    registry.delete(node.id);
    registry.set(replacement.id, replacement);
  };
  return node;
}

// A minimal form control, carrying the default/current split the runtime reads.
function control(spec) {
  const node = Object.assign(
    { tagName: "INPUT", type: "text", name: "", value: "", defaultValue: "", focus() { focused = node; } },
    spec,
  );
  return node;
}

function select(name, options, chosen) {
  return {
    tagName: "SELECT",
    name,
    options: options.map((value) => ({
      value,
      selected: chosen.includes(value),
      defaultSelected: false,
    })),
  };
}

// A region's controls are declared alongside it, so capture and restore have
// something to walk without a real DOM.
function region(id, controls, preserved) {
  const node = {
    id,
    controls: controls || [],
    preserved: preserved || [],
    matches: () => false,
    querySelectorAll: (selector) =>
      selector && selector.indexOf("preserve") >= 0 ? node.preserved : node.controls,
    contains: (item) => (controls || []).indexOf(item) >= 0,
  };
  return node;
}

// A preserved region carries a key and, in a real DOM, whatever the browser
// attached to it. Identity is what the test checks.
function island(key, marker) {
  const node = { key, marker, getAttribute: () => key };
  node.replaceWith = (live) => {
    node.replacedBy = live;
  };
  return node;
}

let focused = null;
let alerted = [];
// What the next parsed region should contain, and what the next parsed head
// payload should be. The stub does no HTML parsing, so tests declare them.
let nextControls = [];
let nextPreserved = [];
let headPayload = null;
// A redraw answers with HTML rather than JSON, so it gets its own stub slot.
let textResponse = null;
let streamResponse = null;
let reloaded = false;
const head = { children: [], appendChild(node) { this.children.push(node); },
  querySelectorAll() { return this.children; } };

globalThis.document = {
  baseURI: "https://example.test/search?q=go",
  head,
  title: "",
  get activeElement() { return focused; },
  listeners: {},
  addEventListener(name, handler) {
    this.listeners[name] = handler;
  },
  // The server passes the endpoint namespace on the script tag, so one shared
  // runtime asset works for any configured prefix.
  currentScript: { dataset: { tinybindPrefix: "/internal/tb", tinybindBuild: "rev-abc" } },
  querySelector(selector) {
    const match = /\[data-tb-id="([^"]+)"\]/.exec(selector);
    const found = match && boundaries.get(match[1]);
    if (!found) return null;
    return swapper(boundaries, found);
  },
  getElementById(id) {
    const found = authorRegions.get(id);
    return found ? swapper(authorRegions, found) : null;
  },
  createElement() {
    const template = { content: { firstElementChild: null, children: [] } };
    Object.defineProperty(template, "innerHTML", {
      set(html) {
        // A head payload is a run of tags; a region payload is one element.
        if (headPayload !== null) {
          template.content.children = headPayload;
          headPayload = null;
          return;
        }
        const id = idOf(html);
        if (!id) {
          template.content.firstElementChild = null;
          return;
        }
        const node = region(id, nextControls, nextPreserved);
        nextPreserved = [];
        node.html = html;
        // A reloadable component emits its kind on every render, so a region
        // stays redrawable after the first redraw replaced it.
        const kind = /data-tb-kind="([^"]+)"/.exec(html);
        node.getAttribute = (name) => (name === "data-tb-kind" && kind ? kind[1] : null);
        nextControls = [];
        template.content.firstElementChild = node;
      },
    });
    return template;
  },
};

globalThis.window = {
  location: {
    origin: "https://example.test",
    pathname: "/search",
    search: "?q=go",
    href: "https://example.test/search?q=go",
    assign(href) {
      assigned = href;
    },
    reload() {
      reloaded = true;
    },
  },
  history: {
    state: null,
    entries: [],
    replaceState(state, title, href) {
      this.state = state;
      if (href) this.href = href;
    },
    pushState(state, title, href) {
      this.state = state;
      this.href = href;
      this.entries.push(href);
    },
  },
  scrollY: 120,
  scrolled: null,
  scrollTo(x, y) { this.scrolled = y; },
  addEventListener() {},
  alert(message) { alerted.push(message); },
};
globalThis.setTimeout = (fn) => fn;
globalThis.TextDecoder = class {
  decode(value) {
    return value && value.length ? Buffer.from(value).toString("utf8") : "";
  }
};

// A streamed body handed out one chunk at a time, so the runtime's line
// buffering and terminator handling are exercised the way a network delivers.
function ndjson(lines) {
  const chunks = lines.map((line) => Buffer.from(line + "\n"));
  let index = 0;
  return {
    getReader: () => ({
      read: () =>
        Promise.resolve(
          index < chunks.length ? { value: chunks[index++], done: false } : { done: true },
        ),
    }),
  };
}
globalThis.URLSearchParams = URLSearchParams;

globalThis.fetch = (href, init) => {
  requests.push({ href, headers: init.headers });
  if (streamResponse) {
    const current = streamResponse;
    return Promise.resolve({
      ok: true,
      status: 200,
      headers: {
        get: (name) =>
          name === "Content-Type"
            ? "application/x-ndjson; charset=utf-8"
            : "navigation;v=1",
      },
      body: ndjson(current),
    });
  }
  if (textResponse) {
    const current = textResponse;
    return Promise.resolve({
      ok: current.ok,
      status: current.status,
      headers: { get: () => null },
      text: () => Promise.resolve(current.text),
    });
  }
  const current = response;
  if (current.abort) {
    return new Promise((_, reject) => {
      init.signal.addEventListener("abort", () => {
        const error = new Error("aborted");
        error.name = "AbortError";
        reject(error);
      });
    });
  }
  return Promise.resolve({
    ok: current.ok !== false,
    status: current.status || 200,
    headers: { get: (name) => (current.headers || {})[name] },
    json: () => Promise.resolve(current.body),
  });
};

globalThis.AbortController = class {
  constructor() {
    this.listeners = [];
    this.signal = { addEventListener: (_, fn) => this.listeners.push(fn) };
  }
  abort() {
    this.listeners.forEach((fn) => fn());
  }
};

eval(fs.readFileSync(process.argv[2], "utf8"));
const runtime = globalThis.window.tinybind;

function check(condition, message) {
  if (!condition) {
    console.error("FAIL: " + message);
    process.exit(1);
  }
}

const delta = (ops, manifest) => ({
  ok: true,
  headers: { "X-Tinybind-Render": "navigation;v=1" },
  body: { v: 1, ops, manifest },
});

async function main() {
  check(runtime.endpointPrefix === "/internal/tb", "prefix should come from the script tag");

  // A first update carries no validators, and stores what comes back.
  response = delta(
    [{ kind: "replace", id: "c2", html: '<p data-tb-id="c2">rust</p>' }],
    [{ id: "c1", frame: "f1" }, { id: "c2", frame: "f2" }],
  );
  let result = await runtime.update("/search?q=rust");
  check(result.applied, "first update should apply");
  check(requests[0].headers["X-Tinybind-Render"] === "navigation;v=1", "render header");
  // The build that rendered the page travels with every request, so a server
  // running a different one can answer with a whole document instead.
  check(requests[0].headers["X-Tinybind-Build"] === "rev-abc", "build header");
  check(!requests[0].headers["X-Tinybind-Manifest"], "first request must send no validators");
  check(window.history.href === "https://example.test/search?q=rust", "URL should be replaced");

  // The next one sends exactly what the previous response established.
  response = delta([], [{ id: "c1", frame: "f1" }, { id: "c2", frame: "f2" }]);
  result = await runtime.update("/search?q=rust&page=2");
  check(result.applied, "second update should apply");
  const hints = requests[1].headers["X-Tinybind-Manifest"];
  check(hints === "c1:f1,c2:f2", "validators should round trip, got " + hints);

  // A response that is not a delta, such as one a cache substituted, must not
  // be applied.
  assigned = null;
  response = { ok: true, headers: {}, body: {} };
  result = await runtime.update("/search?q=go");
  check(result.fellBack, "a non-delta response should fall back");
  check(assigned === "https://example.test/search?q=go", "fallback should navigate");

  // A protocol version the runtime does not speak is not applied either.
  assigned = null;
  response = { ok: true, headers: { "X-Tinybind-Render": "navigation;v=99" }, body: { v: 99 } };
  result = await runtime.update("/search?q=go");
  check(result.fellBack, "a future version should fall back");

  // A superseded request must not overwrite newer state.
  response = { abort: true };
  const stale = runtime.update("/search?q=slow");
  response = delta([], [{ id: "c1", frame: "f3" }]);
  const fresh = runtime.update("/search?q=fast");
  check((await stale).superseded, "the aborted request should report supersession");
  check((await fresh).applied, "the newer request should apply");

  // An action response installs the regions the server chose, whatever the
  // status says, because a rejected submission carries its own error markup.
  const actionResponse = (status, body) => ({
    ok: status < 400,
    status,
    headers: { get: (name) => (name === "X-Tinybind-Render" ? "action;v=1" : null) },
    json: () => Promise.resolve(body),
  });
  result = await runtime.apply(
    actionResponse(422, { v: 1, ops: [{ kind: "replace", id: "cart", html: '<span id="cart">9</span>' }] }),
  );
  check(result.applied, "an action response should apply despite a 4xx status");
  check(authorRegions.get("cart").html === '<span id="cart">9</span>', "the region should be swapped");

  // An action carries no manifest, so it must leave navigation state alone
  // apart from the region it rewrote.
  check(runtime.updateHeaders()["X-Tinybind-Render"] === "action;v=1", "action headers");
  check(runtime.updateHeaders()["X-Tinybind-Build"] === "rev-abc", "action build header");

  // A response that is not an update must not be mistaken for one.
  result = await runtime.apply({
    ok: true,
    status: 200,
    headers: { get: () => null },
    json: () => Promise.resolve({ count: 3 }),
  });
  check(!result.applied, "an ordinary JSON response should not be applied");

  // Typed text survives an update whose server default did not change, and the
  // focus and caret come back with it.
  const typed = control({ name: "q", value: "rust", defaultValue: "", focus() { focused = typed; } });
  typed.selectionStart = 2;
  typed.selectionEnd = 2;
  const chooser = select("sort", ["date", "score"], ["score"]);
  boundaries.set("c2", region("c2", [typed, chooser]));
  focused = typed;

  const reborn = control({ name: "q", value: "", defaultValue: "" });
  nextControls = [reborn, select("sort", ["date", "score"], [])];
  response = delta([{ kind: "replace", id: "c2", html: '<p data-tb-id="c2"></p>' }], []);
  result = await runtime.update("/search?q=rust");
  check(result.applied, "the reconciling update should apply");
  check(reborn.value === "rust", "typed text should survive, got " + reborn.value);
  check(focused === reborn, "focus should return to the same control");
  const restoredSort = boundaries.get("c2").controls[1];
  check(restoredSort.options[1].selected, "the chosen option should survive");

  // A server that changed the default is asserting a new value and wins.
  const edited = control({ name: "q", value: "mine", defaultValue: "old" });
  boundaries.set("c2", region("c2", [edited]));
  focused = null;
  const asserted = control({ name: "q", value: "server", defaultValue: "new" });
  nextControls = [asserted];
  response = delta([{ kind: "replace", id: "c2", html: '<p data-tb-id="c2"></p>' }], []);
  await runtime.update("/search?q=x");
  check(asserted.value === "server", "a changed default should win, got " + asserted.value);

  // A choice that no longer exists yields to the server and is reported rather
  // than vanishing silently.
  alerted = [];
  boundaries.set("c2", region("c2", [select("sort", ["date", "score"], ["score"])]));
  const narrowed = select("sort", ["date"], ["date"]);
  nextControls = [narrowed];
  response = delta([{ kind: "replace", id: "c2", html: '<p data-tb-id="c2"></p>' }], []);
  await runtime.update("/search?q=y");
  check(narrowed.options[0].selected, "the server render should stand");
  check(alerted.length === 1, "a dropped value should be reported, got " + alerted.length);

  // A navigation installs head contributions, retitles the document, pushes
  // history, and resets scroll.
  boundaries.set("c2", region("c2", []));
  headPayload = [
    { tagName: "TITLE", textContent: "Guides", getAttributeNames: () => [], getAttribute: () => null },
    { tagName: "LINK", rel: "", getAttributeNames: () => ["href"], getAttribute: () => "/a.css" },
  ];
  response = {
    ok: true,
    headers: { "X-Tinybind-Render": "navigation;v=1" },
    body: { v: 1, ops: [], manifest: [], head: ["<title>Guides</title>", '<link href="/a.css">'] },
  };
  result = await runtime.navigate("/guides/intro");
  check(result.applied, "navigate should apply");
  check(document.title === "Guides", "the document title should follow the page");
  check(head.children.length === 1, "the new stylesheet should be installed");
  check(
    window.history.entries[window.history.entries.length - 1] === "https://example.test/guides/intro",
    "navigate should push history",
  );
  check(window.scrolled === 0, "a new navigation should reset scroll");

  // Subscribers see the outcome of each navigation, which is what a progress
  // indicator and a widget that must reinitialize both need.
  const seen = [];
  const unsubscribe = runtime.subscribe((event) => seen.push(event.kind));
  response = delta([], []);
  await runtime.update("/search?q=events");
  check(seen[0] === "start" && seen.includes("applied"), "want start then applied, got " + seen);
  unsubscribe();
  const after = seen.length;
  await runtime.update("/search?q=quiet");
  check(seen.length === after, "unsubscribing should stop delivery");

  // A GET form is a navigation whose URL is its own fields.
  globalThis.FormData = class {
    constructor(target) {
      this.entries = target.fields;
    }
    [Symbol.iterator]() {
      return this.entries[Symbol.iterator]();
    }
  };
  const form = { method: "get", action: "https://example.test/search", closest: () => null, fields: [["q", "typed"]] };
  response = delta([], []);
  const submitted = requests.length;
  let prevented = false;
  document.listeners.submit({ target: form, defaultPrevented: false, preventDefault: () => { prevented = true; } });
  await new Promise((resolve) => setImmediate(resolve));
  check(prevented, "a GET form submission should be intercepted");
  check(requests.length > submitted, "the form should issue a delta request");
  check(
    requests[requests.length - 1].href === "https://example.test/search?q=typed",
    "the form fields should become the query, got " + requests[requests.length - 1].href,
  );

  // A non-GET submission mutates, so the browser keeps it.
  const post = { method: "post", action: "https://example.test/cart", closest: () => null, fields: [] };
  const beforePost = requests.length;
  let postPrevented = false;
  document.listeners.submit({ target: post, defaultPrevented: false, preventDefault: () => { postPrevented = true; } });
  check(!postPrevented, "a POST form must be left to the browser");
  check(requests.length === beforePost, "a POST form must issue no delta request");

  // A streamed delta applies record by record and rebuilds the manifest from
  // what it received.
  boundaries.set("c1", region("c1", []));
  boundaries.set("c2", region("c2", []));
  headPayload = null;
  streamResponse = [
    JSON.stringify({ r: "head", v: 1, head: [] }),
    JSON.stringify({ r: "op", kind: "replace", id: "c2", html: '<p data-tb-id="c2">streamed</p>', frame: "s2" }),
    JSON.stringify({ r: "op", id: "c1", frame: "s1" }),
    JSON.stringify({ r: "end" }),
  ];
  result = await runtime.update("/search?q=stream");
  check(result.applied, "a streamed delta should apply");
  check(boundaries.get("c2").html === '<p data-tb-id="c2">streamed</p>', "the streamed region should land");

  // The next request proves the manifest was rebuilt from the records,
  // including the unchanged boundary that carried a validator and no markup.
  streamResponse = null;
  response = delta([], []);
  await runtime.update("/search?q=after");
  const streamed = requests[requests.length - 1].headers["X-Tinybind-Manifest"];
  check(streamed === "c2:s2,c1:s1" || streamed === "c1:s1,c2:s2", "manifest should come from the records, got " + streamed);

  // A stream that stops without its terminator leaves the state unknown, so the
  // manifest is discarded and the next request is a complete render.
  assigned = null;
  streamResponse = [
    JSON.stringify({ r: "head", v: 1, head: [] }),
    JSON.stringify({ r: "op", kind: "replace", id: "c2", html: '<p data-tb-id="c2">partial</p>', frame: "t2" }),
  ];
  result = await runtime.update("/search?q=cut");
  check(result.fellBack, "a truncated stream should fall back");
  streamResponse = null;
  response = delta([], []);
  await runtime.update("/search?q=again");
  check(
    !requests[requests.length - 1].headers["X-Tinybind-Manifest"],
    "a truncated stream must not leave validators behind",
  );

  // A preserved region is moved into the replacement rather than recreated, so
  // whatever the browser attached to it survives.
  const liveWidget = island("player", "original");
  boundaries.set("c2", region("c2", [], [liveWidget]));
  const hole = island("player", "from server");
  nextPreserved = [hole];
  response = delta([{ kind: "replace", id: "c2", html: '<p data-tb-id="c2"></p>' }], []);
  result = await runtime.update("/search?q=z");
  check(result.applied, "the preserving update should apply");
  check(hole.replacedBy === liveWidget, "the live node should be moved into the hole");

  // A redraw names one registered component and supplies its inputs. The kind
  // travels on the element, so the runtime does not have to be told it.
  const card = region("card", []);
  card.getAttribute = (name) => (name === "data-tb-kind" ? "UserCard@8Qv3n1" : null);
  authorRegions.set("card", card);
  textResponse = {
    ok: true,
    status: 200,
    text: '<span id="card" data-tb-kind="UserCard@8Qv3n1">7</span>',
  };
  nextControls = [];
  result = await runtime.redraw("card", { page: 2 });
  check(result.applied, "redraw should apply");
  const redrawURL = requests[requests.length - 1].href;
  check(
    redrawURL === "/internal/tb/redraw/UserCard%408Qv3n1/card?page=2",
    "redraw should use the configured namespace and the element kind, got " + redrawURL,
  );

  check(requests[requests.length - 1].headers["X-Tinybind-Build"] === "rev-abc", "redraw build header");

  // A kind the server does not publish, and a page from another build, both
  // mean this page is stale rather than the region.
  for (const status of [404, 409]) {
    reloaded = false;
    textResponse = { ok: false, status, text: "" };
    result = await runtime.redraw("card", { page: 2 });
    check(result.fellBack, "status " + status + " should fall back");
    check(reloaded, "status " + status + " should reload the page");
  }

  // An element that is not reloadable is refused without a request.
  const plain = region("plain", []);
  plain.getAttribute = () => null;
  authorRegions.set("plain", plain);
  const beforeRedraw = requests.length;
  result = await runtime.redraw("plain", {});
  check(!result.applied, "a non-reloadable element should be refused");
  check(requests.length === beforeRedraw, "a refused redraw must issue no request");

  // A cross-origin target is a browser navigation, never a delta.
  assigned = null;
  const before = requests.length;
  result = await runtime.update("https://elsewhere.test/page");
  check(result.fellBack, "cross-origin should fall back");
  check(requests.length === before, "cross-origin must issue no delta request");

  console.log("ok");
}

main().catch((error) => {
  console.error("FAIL: " + error.stack);
  process.exit(1);
});
