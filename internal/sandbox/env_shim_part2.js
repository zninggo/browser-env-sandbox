// env_shim_part2.js — Global constructors + WebRTC + utility APIs

(function() {
  'use strict';

  // ── atob / btoa ──
  var chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
  var lookup = {};
  for (var i = 0; i < chars.length; i++) lookup[chars[i]] = i;
  window.atob = function(str) {
    var output = '';
    var buffer = 0;
    var bits = 0;
    str = str.replace(/[^A-Za-z0-9+/=]/g, '');
    for (var i = 0; i < str.length; i++) {
      var c = str[i];
      if (c === '=') break;
      buffer = (buffer << 6) | lookup[c];
      bits += 6;
      if (bits >= 8) {
        bits -= 8;
        output += String.fromCharCode((buffer >> bits) & 0xFF);
      }
    }
    return output;
  };
  window.btoa = function(str) {
    var output = '';
    var buffer = 0;
    var bits = 0;
    for (var i = 0; i < str.length; i++) {
      buffer = (buffer << 8) | str.charCodeAt(i);
      bits += 8;
      while (bits >= 6) {
        bits -= 6;
        output += chars[(buffer >> bits) & 0x3F];
      }
    }
    if (bits > 0) {
      output += chars[(buffer << (6 - bits)) & 0x3F];
    }
    while (output.length % 4) output += '=';
    return output;
  };

  // ── TextEncoder / TextDecoder ──
  window.TextEncoder = function() {
    this.encoding = 'utf-8';
  };
  window.TextEncoder.prototype.encode = function(str) {
    str = str || '';
    var bytes = [];
    for (var i = 0; i < str.length; i++) {
      var c = str.charCodeAt(i);
      if (c < 0x80) bytes.push(c);
      else if (c < 0x800) { bytes.push(0xC0 | (c >> 6), 0x80 | (c & 0x3F)); }
      else if (c < 0xD800 || c >= 0xE000) { bytes.push(0xE0 | (c >> 12), 0x80 | ((c >> 6) & 0x3F), 0x80 | (c & 0x3F)); }
      else {
        i++;
        c = 0x10000 + (((c & 0x3FF) << 10) | (str.charCodeAt(i) & 0x3FF));
        bytes.push(0xF0 | (c >> 18), 0x80 | ((c >> 12) & 0x3F), 0x80 | ((c >> 6) & 0x3F), 0x80 | (c & 0x3F));
      }
    }
    return new Uint8Array(bytes);
  };
  window.TextDecoder = function(encoding) {
    this.encoding = encoding || 'utf-8';
    this.fatal = false;
    this.ignoreBOM = false;
  };
  window.TextDecoder.prototype.decode = function(bytes) {
    if (!bytes) return '';
    if (bytes instanceof ArrayBuffer) bytes = new Uint8Array(bytes);
    var str = '';
    for (var i = 0; i < bytes.length; i++) {
      var b = bytes[i];
      if (b < 0x80) str += String.fromCharCode(b);
      else if (b < 0xE0) {
        str += String.fromCharCode(((b & 0x1F) << 6) | (bytes[++i] & 0x3F));
      } else if (b < 0xF0) {
        str += String.fromCharCode(((b & 0x0F) << 12) | ((bytes[++i] & 0x3F) << 6) | (bytes[++i] & 0x3F));
      } else {
        var cp = ((b & 0x07) << 18) | ((bytes[++i] & 0x3F) << 12) | ((bytes[++i] & 0x3F) << 6) | (bytes[++i] & 0x3F);
        cp -= 0x10000;
        str += String.fromCharCode(0xD800 + (cp >> 10), 0xDC00 + (cp & 0x3FF));
      }
    }
    return str;
  };

  // ── URL / URLSearchParams ──
  // Bug 29 fix: full parse handling IPv6 ([::1]:8080 — split(':') breaks it),
  // user:pass@ credentials, default-port omission, and WHATWG-normalized
  // origin/protocol/host fields.
  window.URL = window.URL || function(url, base) {
    var fullUrl = (base && !/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(String(url))) ? String(base) + String(url) : String(url);
    var parser = { href: fullUrl };
    var match = fullUrl.match(/^([a-zA-Z][a-zA-Z0-9+.-]*):\/\/(?:([^@/]*)@)?([^/?#]*)([^?#]*)(\?[^#]*)?(#.*)?$/);
    if (match) {
      var scheme = match[1].toLowerCase();
      var creds = match[2] || '';
      var hostPart = match[3] || '';
      var isIPv6 = hostPart.charAt(0) === '[';
      var hostname = hostPart;
      var port = '';
      if (isIPv6) {
        // [::1]:8080 → host inside brackets, port outside
        var v6 = hostPart.match(/^\[([^\]]*)\](?::(\d+))?$/);
        if (v6) {
          hostname = '[' + v6[1] + ']';
          port = v6[2] || '';
        }
      } else {
        var hi = hostPart.lastIndexOf(':');
        if (hi >= 0) {
          hostname = hostPart.slice(0, hi);
          port = hostPart.slice(hi + 1);
        }
      }
      var pathname = match[4] || '/';
      if (pathname === '') pathname = '/';
      var defaultPorts = { http: '80', https: '443', ws: '80', wss: '443', ftp: '21' };
      var isDefault = port !== '' && defaultPorts[scheme] === port;
      parser.protocol = scheme + ':';
      parser.username = creds.split(':')[0] || '';
      parser.password = creds.indexOf(':') >= 0 ? creds.slice(creds.indexOf(':') + 1) : '';
      parser.hostname = hostname;
      parser.port = isDefault ? '' : port;
      parser.host = port !== '' && !isDefault ? hostname + ':' + port : hostname;
      parser.pathname = pathname;
      parser.search = match[5] || '';
      parser.hash = match[6] || '';
      parser.origin = scheme + '://' + (isIPv6 ? hostname : parser.host);
    }
    parser.toString = function() { return this.href; };
    parser.toJSON = function() { return this.href; };
    return parser;
  };
  window.URL.createObjectURL = function(blob) { return 'blob:https://example.com/' + (++urlCounter); };
  window.URL.revokeObjectURL = function(url) { delete urlStore[url]; };
  // Bug 30 fix: append preserves multiple values; set overwrites.
  // Internal storage: { name: [val1, val2] }
  window.URLSearchParams = function(init) {
    var params = {};
    if (typeof init === 'string') {
      if (init[0] === '?') init = init.slice(1);
      init.split('&').forEach(function(pair) {
        var kv = pair.split('=');
        var k = decodeURIComponent(kv[0]);
        var v = decodeURIComponent(kv[1] || '');
        if (params[k] === undefined) params[k] = [];
        params[k].push(v);
      });
    }
    this.get = function(name) { return params[name] !== undefined ? params[name][0] : null; };
    this.getAll = function(name) { return params[name] !== undefined ? params[name].slice() : []; };
    this.has = function(name) { return params[name] !== undefined; };
    this.set = function(name, value) { params[name] = [value]; };
    this.append = function(name, value) {
      if (params[name] === undefined) params[name] = [];
      params[name].push(value);
    };
    this.delete = function(name) { delete params[name]; };
    this.toString = function() {
      var parts = [];
      Object.keys(params).forEach(function(k) {
        params[k].forEach(function(v) { parts.push(k + '=' + encodeURIComponent(v)); });
      });
      return parts.join('&');
    };
    this.forEach = function(cb) {
      Object.keys(params).forEach(function(k) {
        params[k].forEach(function(v) { cb(v, k); });
      });
    };
    this.entries = function() {
      var result = [];
      Object.keys(params).forEach(function(k) {
        params[k].forEach(function(v) { result.push([k, v]); });
      });
      return result;
    };
  };

  // ── Blob / URL.createObjectURL ──
  // Stores the actual parts so blob().arrayBuffer()/text() round-trip the
  // bytes (needed by fetch→blob→FileReader binary flows).
  window.Blob = function(parts, options) {
    var chunks = [];
    var total = 0;
    if (parts) {
      for (var i = 0; i < parts.length; i++) {
        var p = parts[i];
        if (typeof p === 'string') {
          var bytes = new Uint8Array(p.length);
          for (var j = 0; j < p.length; j++) bytes[j] = p.charCodeAt(j) & 0xFF;
          chunks.push(bytes); total += bytes.length;
        } else if (p instanceof ArrayBuffer) {
          chunks.push(new Uint8Array(p.slice(0))); total += p.byteLength;
        } else if (p && p.buffer instanceof ArrayBuffer) {
          // TypedArray view: copy its byte range (respects byteOffset/length).
          var view = new Uint8Array(p.buffer, p.byteOffset || 0, p.byteLength || p.length);
          chunks.push(new Uint8Array(view)); total += view.length;
        } else if (p && typeof p.size === 'number') {
          // Blob/File-like: keep as-is, resolved lazily by readers.
          chunks.push(p); total += p.size;
        }
      }
    }
    // Privately hold the flat byte copy (Blob-likes flattened on read).
    var flat = null;
    var ensureFlat = function() {
      if (flat) return flat;
      var out = new Uint8Array(total);
      var off = 0;
      for (var i = 0; i < chunks.length; i++) {
        var c = chunks[i];
        if (c instanceof Uint8Array) { out.set(c, off); off += c.length; }
        else if (typeof c._besBytes === 'function') {
          var b = c._besBytes();
          out.set(b, off); off += b.length;
        }
      }
      flat = out;
      return flat;
    };
    this.size = total;
    this.type = (options && options.type) || '';
    this._besBytes = ensureFlat;
    this.text = function() {
      var bytes = ensureFlat();
      // UTF-8 decode with U+FFFD for malformed sequences (WHATWG blob.text()).
      var s = '';
      for (var i = 0; i < bytes.length; ) {
        var b0 = bytes[i];
        if (b0 < 0x80) { s += String.fromCharCode(b0); i++; }
        else if (b0 >= 0xC2 && b0 < 0xE0 && i + 1 < bytes.length && (bytes[i+1] & 0xC0) === 0x80) {
          s += String.fromCharCode(((b0 & 0x1F) << 6) | (bytes[i+1] & 0x3F)); i += 2;
        }
        else if (b0 >= 0xE0 && b0 < 0xF0 && i + 2 < bytes.length && (bytes[i+1] & 0xC0) === 0x80 && (bytes[i+2] & 0xC0) === 0x80) {
          s += String.fromCharCode(((b0 & 0x0F) << 12) | ((bytes[i+1] & 0x3F) << 6) | (bytes[i+2] & 0x3F)); i += 3;
        }
        else if (b0 >= 0xF0 && b0 < 0xF5 && i + 3 < bytes.length && (bytes[i+1] & 0xC0) === 0x80 && (bytes[i+2] & 0xC0) === 0x80 && (bytes[i+3] & 0xC0) === 0x80) {
          var cp = ((b0 & 0x07) << 18) | ((bytes[i+1] & 0x3F) << 12) | ((bytes[i+2] & 0x3F) << 6) | (bytes[i+3] & 0x3F);
          cp -= 0x10000;
          s += String.fromCharCode(0xD800 + (cp >> 10), 0xDC00 + (cp & 0x3FF));
          i += 4;
        }
        else { s += '\uFFFD'; i++; }
      }
      return Promise.resolve(s);
    };
    this.arrayBuffer = function() {
      var bytes = ensureFlat();
      var ab = new ArrayBuffer(bytes.length);
      new Uint8Array(ab).set(bytes);
      return Promise.resolve(ab);
    };
    this.slice = function(start, end, contentType) {
      var bytes = ensureFlat();
      var s = (start === undefined || start < 0) ? 0 : (start > bytes.length ? bytes.length : start);
      var e = (end === undefined || end > bytes.length) ? bytes.length : (end < 0 ? 0 : end);
      var part = bytes.subarray(s, e);
      var ab = new ArrayBuffer(part.length);
      new Uint8Array(ab).set(part);
      return new Blob([ab], { type: contentType || this.type });
    };
  };
  var urlCounter = 0;
  var urlStore = {};

  // ── FormData ──
  window.FormData = function() {
    var data = {};
    this.append = function(name, value, filename) { data[name] = value; };
    this.delete = function(name) { delete data[name]; };
    this.get = function(name) { return data[name] !== undefined ? data[name] : null; };
    this.has = function(name) { return data[name] !== undefined; };
    this.set = function(name, value) { data[name] = value; };
    this.forEach = function(cb) { Object.keys(data).forEach(function(k) { cb(data[k], k); }); };
    this.entries = function() { return Object.keys(data).map(function(k) { return [k, data[k]]; }); };
  };

  // ── AbortController / AbortSignal ──
  window.AbortController = function() {
    var self = this;
    this.signal = {
      aborted: false,
      reason: undefined,
      onabort: null,
      addEventListener: function() {}, removeEventListener: function() {},
      throwIfAborted: function() { if (self.signal.aborted) throw new DOMException('Aborted', 'AbortError'); },
    };
    this.abort = function(reason) { this.signal.aborted = true; this.signal.reason = reason; if (this.signal.onabort) this.signal.onabort(); };
  };

  // ── DOMException (stub) ──
  window.DOMException = function(message, name) {
    this.message = message;
    this.name = name || 'Error';
    this.code = 0;
  };
  window.DOMException.prototype = Object.create(Error.prototype);

  // ── matchMedia ──
  window.matchMedia = function(query) {
    return {
      media: query,
      matches: false,
      onchange: null,
      addListener: function() {},
      removeListener: function() {},
      addEventListener: function() {},
      removeEventListener: function() {},
      dispatchEvent: function() { return true; },
    };
  };

  // ── CSS ──
  window.CSS = {
    supports: function(prop, value) {
      if (arguments.length === 1) return false;
      return true; // Optimistic: say everything is supported
    },
    escape: function(str) { return str.replace(/[^a-zA-Z0-9_-]/g, function(c) { return '\\' + c; }); },
  };

  // ── structuredClone ──
  window.structuredClone = function(obj) {
    if (obj === null || obj === undefined) return obj;
    return JSON.parse(JSON.stringify(obj));
  };

  // ── DOMParser ──
  window.DOMParser = function() {};
  window.DOMParser.prototype.parseFromString = function(str, type) {
    // Return a minimal document-like object
    return {
      documentElement: { tagName: 'html', children: [] },
      head: { children: [] },
      body: { children: [] },
      getElementById: function() { return null; },
      querySelector: function() { return null; },
      querySelectorAll: function() { return []; },
      getElementsByTagName: function() { return []; },
    };
  };

  // ── Notification ──
  window.Notification = function(title, options) {
    this.title = title;
    this.body = (options && options.body) || '';
  };
  window.Notification.permission = 'default';
  window.Notification.requestPermission = function() { return Promise.resolve('denied'); };
  window.Notification.prototype.close = function() {};

  // ── WebRTC (RTCPeerConnection) ──
  window.RTCPeerConnection = function(config) {
    this.localDescription = null;
    this.remoteDescription = null;
    this.iceConnectionState = 'new';
    this.connectionState = 'new';
    this.signalingState = 'stable';
    this.iceGatheringState = 'new';
    this.onicecandidate = null;
    this.ontrack = null;
    this.ondatachannel = null;
    this.oniceconnectionstatechange = null;
    this.onconnectionstatechange = null;
    this.onsignalingstatechange = null;
  };
  window.RTCPeerConnection.prototype.createOffer = function() { return Promise.resolve({ type: 'offer', sdp: '' }); };
  window.RTCPeerConnection.prototype.createAnswer = function() { return Promise.resolve({ type: 'answer', sdp: '' }); };
  window.RTCPeerConnection.prototype.setLocalDescription = function() { return Promise.resolve(); };
  window.RTCPeerConnection.prototype.setRemoteDescription = function() { return Promise.resolve(); };
  window.RTCPeerConnection.prototype.addIceCandidate = function() { return Promise.resolve(); };
  window.RTCPeerConnection.prototype.close = function() {};
  window.RTCPeerConnection.prototype.getStats = function() { return Promise.resolve(new Map()); };
  window.RTCPeerConnection.prototype.createDataChannel = function(label) {
    return { label: label, readyState: 'connecting', send: function() {}, close: function() {},
             onopen: null, onmessage: null, onclose: null, onerror: null };
  };
  window.webkitRTCPeerConnection = window.RTCPeerConnection;
  window.mozRTCPeerConnection = window.RTCPeerConnection;

  // ── WebSocket ──
  var wsCounter = 0;
  window.WebSocket = function(url, protocols) {
    var id = ++wsCounter;
    this.url = url;
    this.readyState = 0; // CONNECTING
    this.protocol = '';
    this.extensions = '';
    this.bufferedAmount = 0;
    this.binaryType = 'blob';
    this.onopen = null;
    this.onmessage = null;
    this.onclose = null;
    this.onerror = null;
    this.send = function(data) { /* stub: real impl via netlayer */ };
    this.close = function() { this.readyState = 3; if (this.onclose) this.onclose({ code: 1000, reason: '' }); };
  };
  window.WebSocket.CONNECTING = 0;
  window.WebSocket.OPEN = 1;
  window.WebSocket.CLOSING = 2;
  window.WebSocket.CLOSED = 3;

  // ── Worker (stub) ──
  window.Worker = function(url, options) {
    this.onmessage = null;
    this.onerror = null;
    this.postMessage = function() {};
    this.terminate = function() {};
    this.addEventListener = function() {};
    this.removeEventListener = function() {};
  };

  // ── BroadcastChannel (enhance existing stub) ──
  // Already defined in env builder, but enhance it

  // ── ResizeObserver ──
  window.ResizeObserver = function(cb) {
    this.observe = function() {};
    this.unobserve = function() {};
    this.disconnect = function() {};
  };

  // ── IntersectionObserver ──
  window.IntersectionObserver = function(cb, options) {
    this.root = null;
    this.rootMargin = '0px';
    this.thresholds = [0];
    this.observe = function() {};
    this.unobserve = function() {};
    this.disconnect = function() {};
    this.takeRecords = function() { return []; };
  };

  // ── Battery API ──
  Object.defineProperty(navigator, 'getBattery', {
    value: function() {
      return Promise.resolve({
        charging: true,
        chargingTime: 0,
        dischargingTime: Infinity,
        level: 1,
        onchargingchange: null,
        onlevelchange: null,
        addEventListener: function() {},
        removeEventListener: function() {},
      });
    },
    configurable: true
  });

  // ── indexedDB ──
  window.indexedDB = {
    open: function(name, version) {
      var req = { result: null, error: null, onsuccess: null, onerror: null, onupgradeneeded: null };
      setTimeout(function() { if (req.onerror) req.error({ target: req }); }, 0);
      return req;
    },
    deleteDatabase: function(name) {
      var req = { onsuccess: null, onerror: null };
      setTimeout(function() { if (req.onsuccess) req.onsuccess({ target: req }); }, 0);
      return req;
    },
    cmp: function(a, b) { return a < b ? -1 : (a > b ? 1 : 0); },
  };
  window.IDBKeyRange = {
    only: function(val) { return { lower: val, upper: val, lowerOpen: false, upperOpen: false }; },
    lowerBound: function(val, open) { return { lower: val, upper: undefined, lowerOpen: !!open, upperOpen: true }; },
    upperBound: function(val, open) { return { lower: undefined, upper: val, lowerOpen: true, upperOpen: !!open }; },
    bound: function(lower, upper, lowerOpen, upperOpen) { return { lower: lower, upper: upper, lowerOpen: !!lowerOpen, upperOpen: !!upperOpen }; },
  };

})();
