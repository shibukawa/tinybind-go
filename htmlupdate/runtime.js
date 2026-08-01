// tinybind partial update runtime.
//
// The protocol details below are hardcoded on purpose: the runtime is shipped
// per framework, so a deployment overriding the attribute prefix or the header
// namespace uses a runtime built for those names. The only negotiated axis is
// the protocol version in the render header.
(function () {
  "use strict";

  var VERSION = 1;
  var ID_ATTR = "data-tb-id";
  var RENDER_HEADER = "X-Tinybind-Render";
  var MANIFEST_HEADER = "X-Tinybind-Manifest";

  // The endpoint namespace is a deployment choice rather than a protocol
  // detail, so the server passes it on the script tag and one shared runtime
  // asset serves any configured prefix.
  // Subscribers watch navigation and redraw progress: a progress indicator, an
  // analytics call, a third-party widget that must reinitialize after its
  // region was replaced. Events carry outcomes, never component arguments.
  var subscribers = [];

  function emit(kind, detail) {
    for (var i = 0; i < subscribers.length; i++) {
      try {
        subscribers[i](Object.assign({ kind: kind }, detail));
      } catch (ignored) {
        // A failing subscriber must not break the update it is watching.
      }
    }
  }

  function subscribe(handler) {
    subscribers.push(handler);
    return function () {
      var at = subscribers.indexOf(handler);
      if (at >= 0) subscribers.splice(at, 1);
    };
  }

  var script = document.currentScript;
  var PREFIX = (script && script.dataset && script.dataset.tinybindPrefix) || "/_tb";
  // The build that rendered this page. A server running a different one cannot
  // vouch for the state this page holds, so it answers with a whole document
  // and the page reloads rather than patching across the change.
  var BUILD = (script && script.dataset && script.dataset.tinybindBuild) || "";
  var BUILD_HEADER = "X-Tinybind-Build";

  // Validators are held in memory rather than in the DOM, because a boundary's
  // start tag is written before its content exists and so cannot carry a digest
  // of it. An empty map simply means the first update returns every boundary.
  var validators = new Map();
  var inFlight = null;
  var sequence = 0;

  function manifestHeader() {
    var parts = [];
    validators.forEach(function (frame, id) {
      parts.push(id + ":" + frame);
    });
    return parts.join(",");
  }

  // A delta reuses the live document shell, so a component appearing for the
  // first time has no stylesheet installed. Its markup must not land before the
  // sheet does, or the region flashes unstyled.
  function syncHead(tags) {
    if (!tags || !tags.length) return Promise.resolve();
    var head = document.head;
    var present = new Set();
    var existing = head.querySelectorAll("link,script,meta,title");
    for (var i = 0; i < existing.length; i++) present.add(identity(existing[i]));
    var pending = [];
    var template = document.createElement("template");
    template.innerHTML = tags.join("");
    var incoming = Array.prototype.slice.call(template.content.children);
    for (var j = 0; j < incoming.length; j++) {
      var node = incoming[j];
      // A title is a singleton: the new one replaces the old rather than
      // joining it, or history and assistive technology see the old page.
      if (node.tagName === "TITLE") {
        document.title = node.textContent;
        continue;
      }
      if (present.has(identity(node))) continue;
      head.appendChild(node);
      if (node.tagName === "LINK" && node.rel === "stylesheet") pending.push(node);
    }
    if (!pending.length) return Promise.resolve();
    // A sheet that never loads must not stall the update forever.
    return Promise.race([
      Promise.all(pending.map(loaded)),
      new Promise(function (resolve) { setTimeout(resolve, HEAD_TIMEOUT_MS); }),
    ]);
  }

  var HEAD_TIMEOUT_MS = 3000;

  // Identity matches the server's own head merging: element name plus its
  // normalized attributes. Contributions are never removed, because reference
  // counting them across independently updated regions is error prone and a
  // content-hashed asset URL is inert once loaded.
  function identity(node) {
    var names = node.getAttributeNames ? node.getAttributeNames().slice().sort() : [];
    var parts = [node.tagName];
    for (var i = 0; i < names.length; i++) parts.push(names[i] + "=" + node.getAttribute(names[i]));
    return parts.join("\u0000");
  }

  function loaded(node) {
    return new Promise(function (resolve) {
      node.addEventListener("load", resolve, { once: true });
      node.addEventListener("error", resolve, { once: true });
    });
  }

  // A navigation addresses an automatic boundary by its framework attribute; a
  // redraw or an action addresses a region by the id its author wrote. Both
  // namespaces are server-controlled, so trying each in turn is unambiguous.
  function resolve(id) {
    return (
      document.querySelector("[" + ID_ATTR + '="' + id + '"]') ||
      document.getElementById(id)
    );
  }

  // Client state comes in two kinds. Focus and text selection belong to a node
  // and survive only if it does. A chosen option, a checked box, and typed text
  // are keyed to a value, so they can outlive a replacement. HTML already
  // separates what the server said from what the user did, through the default
  // properties, so detecting the second kind needs no bookkeeping at all.

  function controlsIn(root) {
    var found = [];
    if (root.matches && root.matches("input,textarea,select")) found.push(root);
    var nested = root.querySelectorAll ? root.querySelectorAll("input,textarea,select") : [];
    for (var i = 0; i < nested.length; i++) found.push(nested[i]);
    return found;
  }

  function keyOf(control, index) {
    return control.name || "\u0000" + index;
  }

  function isSelect(control) { return control.tagName === "SELECT"; }
  function isToggle(control) { return control.type === "checkbox" || control.type === "radio"; }

  function currentOf(control) {
    if (isToggle(control)) return control.checked;
    if (isSelect(control)) {
      var chosen = [];
      for (var i = 0; i < control.options.length; i++) {
        if (control.options[i].selected) chosen.push(control.options[i].value);
      }
      return chosen;
    }
    return control.value;
  }

  function defaultOf(control) {
    if (isToggle(control)) return String(control.defaultChecked);
    if (isSelect(control)) {
      var defaults = [];
      for (var i = 0; i < control.options.length; i++) {
        if (control.options[i].defaultSelected) defaults.push(control.options[i].value);
      }
      return defaults.join("\u0000");
    }
    return control.defaultValue;
  }

  function isDirty(control) {
    // A file selection can be neither captured nor restored, so it is never
    // treated as recoverable state.
    if (control.type === "file") return false;
    if (isToggle(control)) return control.checked !== control.defaultChecked;
    if (isSelect(control)) {
      for (var i = 0; i < control.options.length; i++) {
        if (control.options[i].selected !== control.options[i].defaultSelected) return true;
      }
      return false;
    }
    return control.value !== control.defaultValue;
  }

  function capture(root) {
    var controls = controlsIn(root);
    var records = [];
    for (var i = 0; i < controls.length; i++) {
      if (!isDirty(controls[i])) continue;
      records.push({
        key: keyOf(controls[i], i),
        value: currentOf(controls[i]),
        was: defaultOf(controls[i]),
      });
    }
    var active = document.activeElement;
    var focus = null;
    for (var j = 0; j < controls.length; j++) {
      if (controls[j] !== active) continue;
      focus = { key: keyOf(controls[j], j), start: active.selectionStart, end: active.selectionEnd };
    }
    return { records: records, focus: focus };
  }

  // restore puts a user value back only where it still applies and the server
  // stayed silent. A changed default means the server is asserting a new value
  // and wins; an unchanged one means it expressed no opinion, and treating that
  // silence as an assertion would clear typed text on every unrelated update.
  function restore(root, state) {
    var controls = controlsIn(root);
    var byKey = new Map();
    for (var i = 0; i < controls.length; i++) byKey.set(keyOf(controls[i], i), controls[i]);
    for (var j = 0; j < state.records.length; j++) {
      var record = state.records[j];
      var control = byKey.get(record.key);
      if (!control) continue;
      if (defaultOf(control) !== record.was) continue;
      if (!putValue(control, record.value)) {
        // Provisional: a dropped choice is silent data loss, so it is made
        // impossible to miss until an application-facing event replaces this.
        window.alert("tinybind: a value no longer applies and was discarded: " + record.key);
      }
    }
    if (!state.focus) return;
    var target = byKey.get(state.focus.key);
    if (!target || !target.focus) return;
    target.focus();
    if (state.focus.start == null || !target.setSelectionRange) return;
    try {
      target.setSelectionRange(state.focus.start, state.focus.end);
    } catch (ignored) {
      // A control whose type forbids selection ranges keeps the focus alone.
    }
  }

  // putValue reports whether the value still applied. A select is checked
  // before anything is mutated, so a value that no longer exists leaves the
  // server's render in place rather than clearing the control.
  function putValue(control, value) {
    if (isToggle(control)) {
      control.checked = value;
      return true;
    }
    if (isSelect(control)) {
      var available = new Set();
      for (var i = 0; i < control.options.length; i++) available.add(control.options[i].value);
      for (var j = 0; j < value.length; j++) {
        if (!available.has(value[j])) return false;
      }
      var wanted = new Set(value);
      for (var k = 0; k < control.options.length; k++) {
        control.options[k].selected = wanted.has(control.options[k].value);
      }
      return true;
    }
    control.value = value;
    return true;
  }

  var PRESERVE_ATTR = "data-tinybind-preserve";

  // A preserved region is one the server cannot patch: a third-party widget, a
  // canvas, a media element it does not own. The runtime moves the live node
  // into the replacement instead of accepting the server's version, so the node
  // and everything the browser attached to it survive.
  function graft(target, replacement) {
    if (!target.querySelectorAll || !replacement.querySelectorAll) return;
    var existing = target.querySelectorAll("[" + PRESERVE_ATTR + "]");
    if (!existing.length) return;
    var byKey = new Map();
    for (var i = 0; i < existing.length; i++) {
      byKey.set(existing[i].getAttribute(PRESERVE_ATTR), existing[i]);
    }
    var holes = replacement.querySelectorAll("[" + PRESERVE_ATTR + "]");
    for (var j = 0; j < holes.length; j++) {
      var live = byKey.get(holes[j].getAttribute(PRESERVE_ATTR));
      // A key with no counterpart is a new region, so the server's version
      // stands rather than being replaced by an unrelated node.
      if (live) holes[j].replaceWith(live);
    }
  }

  // swap installs a replacement in place of a live region, carrying across the
  // state the markup does not describe.
  function swap(target, replacement) {
    var state = capture(target);
    graft(target, replacement);
    target.replaceWith(replacement);
    restore(replacement, state);
  }

  function applyOps(body) {
    var ops = body.ops || [];
    for (var i = 0; i < ops.length; i++) {
      var op = ops[i];
      var target = resolve(op.id);
      // A missing target means the document drifted from the manifest. Losing
      // the whole update is better than applying half of it.
      if (!target) return false;
      if (op.kind !== "replace") return false;
      var template = document.createElement("template");
      template.innerHTML = op.html;
      var replacement = template.content.firstElementChild;
      if (!replacement) return false;
      swap(target, replacement);
      // The region no longer matches the validator held for it. Keeping the old
      // one would let a later navigation call the boundary unchanged and leave
      // this markup in place forever.
      validators.delete(op.id);
    }
    // A navigation restates the whole manifest; an action carries none and must
    // leave the rest of the state alone.
    if (body.manifest) {
      validators.clear();
      for (var j = 0; j < body.manifest.length; j++) {
        validators.set(body.manifest[j].id, body.manifest[j].frame);
      }
    }
    return true;
  }

  // apply installs head contributions before content, so a region whose
  // stylesheet just arrived is never painted unstyled.
  function apply(body) {
    if (body.v !== VERSION) return Promise.resolve(false);
    if (body.navigate) {
      window.location.assign(body.navigate);
      return Promise.resolve(true);
    }
    return syncHead(body.head).then(function () { return applyOps(body); });
  }

  // go fetches a URL as a delta and applies it. Everything the runtime cannot
  // handle performs the ordinary browser navigation instead, so a user action
  // is never silently lost.
  //
  // History is written after the response commits rather than before, so a
  // failed delta leaves the history stack untouched.
  function go(url, options) {
    var target = new URL(url, document.baseURI);
    if (target.origin !== window.location.origin) {
      window.location.assign(target.href);
      return Promise.resolve({ fellBack: true });
    }
    var history = (options && options.history) || "replace";
    var scroll = (options && options.scroll) || "none";
    var restoreTo = options && options.restoreTo;
    // Record where the page was before leaving, so back and forward can put it
    // back exactly.
    rememberScroll();
    emit("start", { url: target.href });
    var ticket = ++sequence;
    if (inFlight) inFlight.abort();
    var controller = new AbortController();
    inFlight = controller;
    var headers = {};
    headers[RENDER_HEADER] = "navigation;v=" + VERSION;
    headers[BUILD_HEADER] = BUILD;
    var hints = manifestHeader();
    if (hints) headers[MANIFEST_HEADER] = hints;
    return fetch(target.href, {
      headers: headers,
      credentials: "same-origin",
      signal: controller.signal,
    })
      .then(function (response) {
        if (!response.ok) throw new Error("update failed: " + response.status);
        var served = response.headers.get(RENDER_HEADER);
        // A cache or proxy may have answered with the document body instead.
        if (served !== "navigation;v=" + VERSION) throw new Error("not a delta");
        var type = response.headers.get("Content-Type") || "";
        if (type.indexOf(STREAM_TYPE) >= 0 && response.body && response.body.getReader) {
          return consumeStream(response).then(function () { return null; });
        }
        return response.json();
      })
      .then(function (body) {
        // A superseded navigation must not overwrite newer state.
        if (ticket !== sequence) return { superseded: true };
        // A streamed delta already applied itself record by record.
        return (body === null ? Promise.resolve(true) : apply(body)).then(function (ok) {
          if (ticket !== sequence) return { superseded: true };
          if (!ok) throw new Error("could not apply delta");
          if (history === "push") {
            window.history.pushState({ y: 0 }, "", target.href);
          } else if (history === "replace") {
            window.history.replaceState(window.history.state, "", target.href);
          }
          applyScroll(scroll, restoreTo, target.hash);
          emit("applied", { url: target.href });
          return { applied: true };
        });
      })
      .catch(function (error) {
        if (error && error.name === "AbortError") {
          emit("superseded", { url: target.href });
          return { superseded: true };
        }
        emit("fellBack", { url: target.href, reason: String(error && error.message) });
        window.location.assign(target.href);
        return { fellBack: true };
      });
  }

  function rememberScroll() {
    if (!window.history.replaceState) return;
    var state = window.history.state || {};
    state.y = window.scrollY || 0;
    window.history.replaceState(state, "");
  }

  function applyScroll(mode, restoreTo, hash) {
    if (mode === "none") return;
    if (mode === "restore") {
      window.scrollTo(0, restoreTo || 0);
      return;
    }
    // A fragment target wins over the top of the page, and it is resolved after
    // the operations landed so the element exists.
    if (hash) {
      var anchor = document.getElementById(hash.slice(1));
      if (anchor && anchor.scrollIntoView) {
        anchor.scrollIntoView();
        return;
      }
    }
    window.scrollTo(0, 0);
  }

  // update re-renders the current route, which is what a search-parameter
  // change needs: the URL is corrected in place rather than pushed.
  function update(url) {
    return go(url, { history: "replace", scroll: "none" });
  }

  // navigate moves to another route, pushing history and resetting scroll the
  // way a real navigation would.
  function navigate(url, options) {
    return go(url, {
      history: (options && options.history) || "push",
      scroll: (options && options.scroll) || "top",
    });
  }

  // Link and form interception. Only a plain same-origin GET is taken: a
  // modified click, a target, a download, and a cross-origin URL all belong to
  // the browser, and an opt-out attribute returns any element to it.
  function intercept() {
    if (!document.addEventListener) return;
    document.addEventListener("click", function (event) {
      if (event.defaultPrevented || event.button !== 0) return;
      if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      var link = event.target.closest && event.target.closest("a[href]");
      if (!link || link.target || link.hasAttribute("download")) return;
      if (link.closest("[data-tinybind-ignore]")) return;
      var target = new URL(link.href, document.baseURI);
      if (target.origin !== window.location.origin) return;
      // A same-document fragment is the browser's job.
      if (target.pathname === window.location.pathname &&
          target.search === window.location.search && target.hash) return;
      event.preventDefault();
      navigate(target.href);
    });
    // A GET form is a navigation whose URL is its own fields, so it is
    // intercepted the same way a link is. A non-GET submission mutates and is
    // left to the browser, which is what keeps post-redirect-get working.
    document.addEventListener("submit", function (event) {
      if (event.defaultPrevented) return;
      var form = event.target;
      if (!form || (form.method || "get").toLowerCase() !== "get") return;
      if (form.closest && form.closest("[data-tinybind-ignore]")) return;
      var action = new URL(form.action || window.location.href, document.baseURI);
      if (action.origin !== window.location.origin) return;
      event.preventDefault();
      action.search = new URLSearchParams(new FormData(form)).toString();
      // A search form refines the page it is on, so its URL replaces rather
      // than stacking a history entry per keystroke-driven submit.
      var samePage = action.pathname === window.location.pathname;
      go(action.href, {
        history: samePage ? "replace" : "push",
        scroll: samePage ? "none" : "top",
      });
    });
    window.addEventListener("popstate", function (event) {
      var state = event.state || {};
      go(window.location.href, { history: "none", scroll: "restore", restoreTo: state.y });
    });
  }

  // redraw re-renders one registered component, naming it by the id its author
  // wrote and supplying every declared parameter. Nothing is reconstructed on
  // the server, so the response is that component's subtree and nothing else.
  function redraw(elementId, params, options) {
    var target = document.getElementById(elementId);
    if (!target) {
      return Promise.resolve({ applied: false, reason: "no such element" });
    }
    var kind = target.getAttribute(KIND_ATTR);
    if (!kind) {
      return Promise.resolve({ applied: false, reason: "not a reloadable component" });
    }
    var ticket = (redrawTickets.get(elementId) || 0) + 1;
    redrawTickets.set(elementId, ticket);
    var query = new URLSearchParams();
    Object.keys(params || {}).forEach(function (name) {
      query.set(name, String(params[name]));
    });
    var url = PREFIX + "/redraw/" + encodeURIComponent(kind) + "/" + encodeURIComponent(elementId);
    var suffix = query.toString();
    if (suffix) url += "?" + suffix;
    var headers = {};
    headers[BUILD_HEADER] = BUILD;
    return fetch(url, { credentials: "same-origin", headers: headers })
      .then(function (response) {
        // A kind this server does not know, or a build that is not this one,
        // means the page itself is stale rather than the region.
        if (response.status === 404 || response.status === 409) {
          throw new Error("stale page");
        }
        if (!response.ok) throw new Error("redraw failed: " + response.status);
        return response.text();
      })
      .then(function (html) {
        // A superseded redraw of the same region must not overwrite newer state.
        if (redrawTickets.get(elementId) !== ticket) return { superseded: true };
        var current = document.getElementById(elementId);
        if (!current) return { applied: false, reason: "target disappeared" };
        var template = document.createElement("template");
        template.innerHTML = html;
        var replacement = template.content.firstElementChild;
        if (!replacement) return { applied: false, reason: "empty fragment" };
        swap(current, replacement);
        validators.delete(elementId);
        emit("redrawn", { id: elementId });
        return { applied: true };
      })
      .catch(function () {
        window.location.reload();
        return { fellBack: true };
      });
  }

  var KIND_ATTR = "data-tb-kind";
  var redrawTickets = new Map();

  var STREAM_TYPE = "application/x-ndjson";

  // A streamed delta arrives as one JSON record per line. Each record carries
  // its own manifest entry, because a trailing manifest cannot be written
  // before the operations it describes.
  //
  // The stream ends with an explicit terminator. One that stops without it
  // leaves the manifest state unknown, so it is discarded and the next request
  // is a complete render rather than a delta built on a guess.
  function consumeStream(response) {
    var reader = response.body.getReader();
    var decoder = new TextDecoder();
    var buffered = "";
    var next = new Map();
    var complete = false;
    var failed = false;
    // Records are applied in order through one chain. Failure is tracked in a
    // flag rather than threaded as a resolved value, because a step that
    // resolves with nothing would otherwise read as a failure.
    var chain = Promise.resolve();

    function handle(line) {
      if (!line) return;
      var item = JSON.parse(line);
      if (item.r === "head") {
        if (item.v !== VERSION) throw new Error("version mismatch");
        chain = chain.then(function () { return syncHead(item.head); });
        return;
      }
      if (item.r === "end") {
        complete = true;
        return;
      }
      if (item.r === "await") {
        // An await completion addresses a placeholder inside a region already
        // installed, so it replaces that element rather than a boundary.
        chain = chain.then(function () {
          if (failed) return;
          var placeholder = document.getElementById(item.id);
          if (!placeholder) return;
          var template = document.createElement("template");
          template.innerHTML = item.html;
          var settled = template.content.firstElementChild;
          if (settled) placeholder.replaceWith(settled);
        });
        return;
      }
      if (item.r !== "op") return;
      next.set(item.id, item.frame);
      // An entry with no markup is an unchanged boundary restating its
      // validator, so there is nothing to apply.
      if (!item.html) return;
      chain = chain.then(function () {
        if (failed) return;
        if (!applyOps({ v: VERSION, ops: [item] })) failed = true;
      });
    }

    function pump() {
      return reader.read().then(function (chunk) {
        buffered += decoder.decode(chunk.value || new Uint8Array(), { stream: !chunk.done });
        var lines = buffered.split("\n");
        buffered = lines.pop();
        for (var i = 0; i < lines.length; i++) handle(lines[i].trim());
        if (!chunk.done) return pump();
        handle(buffered.trim());
        return chain;
      });
    }

    return pump().then(function () {
      if (!complete) {
        // Applied operations are not rolled back; the state is simply unknown.
        validators.clear();
        throw new Error("truncated stream");
      }
      if (failed) throw new Error("could not apply delta");
      validators.clear();
      next.forEach(function (frame, id) {
        if (frame) validators.set(id, frame);
      });
      return true;
    });
  }

  // live keeps a delivery stream open and reopens it when it drops.
  //
  // Reconnecting is the same request again: a live delivery carries the whole
  // state of its region rather than an increment, so nothing has to be resumed
  // and a missed value costs nothing. What the runtime must not do is retry
  // forever, or a server restart attracts a reconnect storm.
  function live(url, options) {
    var target = new URL(url || window.location.href, document.baseURI);
    var settings = options || {};
    var maxAttempts = settings.maxAttempts || 6;
    var backoff = settings.backoffMs || 500;
    var stopped = false;
    var attempts = 0;

    function open() {
      if (stopped) return Promise.resolve({ stopped: true });
      var headers = {};
      headers[RENDER_HEADER] = "navigation;v=" + VERSION;
      headers[BUILD_HEADER] = BUILD;
      var hints = manifestHeader();
      if (hints) headers[MANIFEST_HEADER] = hints;
      return fetch(target.href, { headers: headers, credentials: "same-origin" })
        .then(function (response) {
          if (!response.ok) throw new Error("live failed: " + response.status);
          if (!response.body || !response.body.getReader) throw new Error("not a stream");
          return consumeStream(response);
        })
        .then(function () {
          // The server finished on purpose, so there is nothing to reconnect to.
          stopped = true;
          emit("liveEnded", { url: target.href });
          return { ended: true };
        })
        .catch(function () {
          attempts++;
          if (stopped || attempts >= maxAttempts) {
            emit("fellBack", { url: target.href, reason: "live stream lost" });
            window.location.reload();
            return { fellBack: true };
          }
          emit("liveReconnecting", { url: target.href, attempt: attempts });
          return new Promise(function (resolve) {
            setTimeout(resolve, backoff * attempts);
          }).then(open);
        });
    }

    var running = open();
    return {
      close: function () {
        stopped = true;
      },
      done: running,
    };
  }

  // updateHeaders is what an application spreads into its own fetch, so the
  // server knows the caller can apply an update response and can still redirect
  // for an ordinary form submission.
  function updateHeaders() {
    var headers = {};
    headers[RENDER_HEADER] = "action;v=" + VERSION;
    headers[BUILD_HEADER] = BUILD;
    return headers;
  }

  // applyResponse takes the Response of an application's own API call and
  // installs whatever regions the server chose to rewrite, so one round trip
  // both performs the action and refreshes the page.
  //
  // Status is deliberately ignored: a rejected submission returns 4xx and the
  // regions it carries are the validation errors, which is exactly what has to
  // be shown.
  function applyResponse(response) {
    var served = response.headers.get(RENDER_HEADER);
    if (served !== "action;v=" + VERSION) {
      return Promise.resolve({ applied: false, reason: "not an update response" });
    }
    return response
      .json()
      .then(apply)
      .then(function (ok) {
        return ok ? { applied: true } : { applied: false, reason: "could not apply" };
      });
  }

  intercept();

  window.tinybind = window.tinybind || {};
  window.tinybind.update = update;
  window.tinybind.navigate = navigate;
  window.tinybind.redraw = redraw;
  window.tinybind.subscribe = subscribe;
  window.tinybind.live = live;
  window.tinybind.apply = applyResponse;
  window.tinybind.updateHeaders = updateHeaders;
  window.tinybind.protocolVersion = VERSION;
  window.tinybind.endpointPrefix = PREFIX;
})();
