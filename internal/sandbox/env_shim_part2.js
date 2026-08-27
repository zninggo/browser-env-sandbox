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
  window.URL = window.URL || function(url, base) {
    var fullUrl = base ? base + url : url;
    var parser = { href: fullUrl };
    // Simple URL parsing
    var match = fullUrl.match(/^([^:]+):\/\/([^/]+)([^?]*)(\?[^#]*)?(#.*)?$/);
    if (match) {
      parser.protocol = match[1] + ':';
      parser.host = match[2];
      parser.hostname = match[2].split(':')[0];
      parser.port = match[2].split(':')[1] || '';
      parser.pathname = match[3] || '/';
      parser.search = match[4] || '';
      parser.hash = match[5] || '';
      parser.origin = match[1] + '://' + match[2];
    }
    parser.toString = function() { return this.href; };
    return parser;
  };
  window.URLSearchParams = function(init) {
    var params = {};
    if (typeof init === 'string') {
      if (init[0] === '?') init = init.slice(1);
      init.split('&').forEach(function(pair) {
        var kv = pair.split('=');
        params[decodeURIComponent(kv[0])] = decodeURIComponent(kv[1] || '');
      });
    }
    this.get = function(name) { return params[name] !== undefined ? params[name] : null; };
    this.getAll = function(name) { return params[name] !== undefined ? [params[name]] : []; };
    this.has = function(name) { return params[name] !== undefined; };
    this.set = function(name, value) { params[name] = value; };
    this.append = function(name, value) { params[name] = value; };
    this.delete = function(name) { delete params[name]; };
    this.toString = function() {
      return Object.keys(params).map(function(k) { return k + '=' + encodeURIComponent(params[k]); }).join('&');
    };
    this.forEach = function(cb) { Object.keys(params).forEach(function(k) { cb(params[k], k); }); };
    this.entries = function() { return Object.keys(params).map(function(k) { return [k, params[k]]; }); };
  };

  // ── Blob / URL.createObjectURL ──
  window.Blob = function(parts, options) {
    this.size = 0;
    this.type = (options && options.type) || '';
    if (parts) {
      for (var i = 0; i < parts.length; i++) {
        var p = parts[i];
        this.size += (typeof p === 'string') ? p.length : (p.byteLength || p.size || 0);
      }
    }
    this.text = function() { return Promise.resolve(parts ? parts.join('') : ''); };
    this.arrayBuffer = function() { return Promise.resolve(new ArrayBuffer(this.size)); };
    this.slice = function() { return new Blob([], { type: this.type }); };
  };
  var urlCounter = 0;
  var urlStore = {};
  window.URL.createObjectURL = function(blob) { return 'blob:https://example.com/' + (++urlCounter); };
  window.URL.revokeObjectURL = function(url) { delete urlStore[url]; };

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
