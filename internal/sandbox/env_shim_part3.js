// env_shim_part3.js — DOM element enhancement + toString spoofing + Symbol.toStringTag

(function() {
  'use strict';

  // ── Enhance createElement to return richer elements ──
  var origCreateElement = document.createElement;
  document.createElement = function(tagName) {
    var el = origCreateElement.call(document, tagName);
    if (!el) return el;
    tagName = String(tagName).toLowerCase();

    // innerHTML getter/setter
    var _innerHTML = '';
    Object.defineProperty(el, 'innerHTML', {
      get: function() { return _innerHTML; },
      set: function(v) { _innerHTML = v; },
      configurable: true,
    });

    // outerHTML
    Object.defineProperty(el, 'outerHTML', {
      get: function() { return _innerHTML; },
      configurable: true,
    });

    // classList
    var _classes = [];
    Object.defineProperty(el, 'classList', {
      value: {
        add: function(c) { if (_classes.indexOf(c) < 0) _classes.push(c); },
        remove: function(c) { var i = _classes.indexOf(c); if (i >= 0) _classes.splice(i, 1); },
        toggle: function(c) { var i = _classes.indexOf(c); if (i >= 0) _classes.splice(i, 1); else _classes.push(c); },
        contains: function(c) { return _classes.indexOf(c) >= 0; },
        item: function(i) { return _classes[i] || null; },
        toString: function() { return _classes.join(' '); },
        get length() { return _classes.length; },
      },
      configurable: true,
    });

    // dataset
    Object.defineProperty(el, 'dataset', {
      value: {},
      configurable: true,
    });

    // offset properties (layout mock)
    Object.defineProperty(el, 'offsetWidth', { value: 0, configurable: true });
    Object.defineProperty(el, 'offsetHeight', { value: 0, configurable: true });
    Object.defineProperty(el, 'offsetTop', { value: 0, configurable: true });
    Object.defineProperty(el, 'offsetLeft', { value: 0, configurable: true });
    Object.defineProperty(el, 'offsetParent', { value: null, configurable: true });
    Object.defineProperty(el, 'clientWidth', { value: 0, configurable: true });
    Object.defineProperty(el, 'clientHeight', { value: 0, configurable: true });
    Object.defineProperty(el, 'clientTop', { value: 0, configurable: true });
    Object.defineProperty(el, 'clientLeft', { value: 0, configurable: true });
    Object.defineProperty(el, 'scrollWidth', { value: 0, configurable: true });
    Object.defineProperty(el, 'scrollHeight', { value: 0, configurable: true });
    Object.defineProperty(el, 'scrollTop', { get: function() { return 0; }, set: function() {}, configurable: true });
    Object.defineProperty(el, 'scrollLeft', { get: function() { return 0; }, set: function() {}, configurable: true });

    // event listeners storage
    var listeners = {};
    el.addEventListener = function(type, handler) {
      if (!listeners[type]) listeners[type] = [];
      listeners[type].push(handler);
    };
    el.removeEventListener = function(type, handler) {
      if (!listeners[type]) return;
      var i = listeners[type].indexOf(handler);
      if (i >= 0) listeners[type].splice(i, 1);
    };
    el.dispatchEvent = function(event) {
      if (listeners[event.type]) {
        listeners[event.type].forEach(function(h) { h(event); });
      }
      return true;
    };

    // click
    el.click = function() { this.dispatchEvent({ type: 'click' }); };

    // focus / blur
    el.focus = function() {};
    el.blur = function() {};

    // contains
    el.contains = function(node) { return false; };

    // querySelector on element
    el.querySelector = function(sel) { return null; };
    el.querySelectorAll = function(sel) { return []; };

    // cloneNode
    el.cloneNode = function(deep) { return origCreateElement.call(document, tagName); };

    // insertAdjacentHTML
    el.insertAdjacentHTML = function(pos, html) {};

    // matches
    el.matches = function(sel) { return false; };
    el.closest = function(sel) { return null; };

    // style getter (enhance existing)
    if (!el.style) {
      Object.defineProperty(el, 'style', { value: { cssText: '', getPropertyValue: function() { return ''; }, setProperty: function() {}, removeProperty: function() {} }, configurable: true });
    }

    // dataset already set above

    return el;
  };

  // ── document.head / document.body enhancement ──
  // Make sure head and body exist as elements
  if (!document.head) {
    document.head = document.createElement('head');
  }
  if (!document.body) {
    document.body = document.createElement('body');
  }
  if (!document.documentElement) {
    document.documentElement = document.createElement('html');
  }

  // ── window.getComputedStyle ──
  window.getComputedStyle = function(el, pseudo) {
    return {
      getPropertyValue: function(prop) { return ''; },
      getPropertyCSSValue: function(prop) { return null; },
      cssText: '',
      length: 0,
      item: function(i) { return ''; },
    };
  };

  // ── Symbol.toStringTag on key objects ──
  try { Object.defineProperty(navigator, Symbol.toStringTag, { value: 'Navigator', configurable: true }); } catch(e) {}
  try { Object.defineProperty(screen, Symbol.toStringTag, { value: 'Screen', configurable: true }); } catch(e) {}
  try { Object.defineProperty(document, Symbol.toStringTag, { value: 'HTMLDocument', configurable: true }); } catch(e) {}
  try { Object.defineProperty(window, Symbol.toStringTag, { value: 'Window', configurable: true }); } catch(e) {}
  try { Object.defineProperty(history, Symbol.toStringTag, { value: 'History', configurable: true }); } catch(e) {}
  try { Object.defineProperty(location, Symbol.toStringTag, { value: 'Location', configurable: true }); } catch(e) {}

  // ── Native function toString spoofing ──
  // Override Function.prototype.toString to return native format for our mocks
  var origFnToString = Function.prototype.toString;
  var nativeFns = new WeakSet();
  Function.prototype.toString = function() {
    if (nativeFns.has(this)) {
      return 'function ' + (this.name || '') + '() { [native code] }';
    }
    return origFnToString.call(this);
  };
  // Mark key functions as native
  try { nativeFns.add(navigator.toString); } catch(e) {}
  try { nativeFns.add(document.createElement); } catch(e) {}
  try { nativeFns.add(document.getElementById); } catch(e) {}
  try { nativeFns.add(window.setTimeout); } catch(e) {}
  try { nativeFns.add(window.setInterval); } catch(e) {}
  try { nativeFns.add(window.fetch); } catch(e) {}
  try { nativeFns.add(performance.now); } catch(e) {}

  // ── window.dispatchEvent ──
  window.dispatchEvent = function(event) { return true; };
  window.addEventListener = function(type, handler) {};
  window.removeEventListener = function(type, handler) {};

  // ── window.requestIdleCallback (enhance existing) ──
  // Already defined in env builder

  // ── crypto.subtle ──
  if (window.crypto && !window.crypto.subtle) {
    Object.defineProperty(window.crypto, 'subtle', {
      value: {
        digest: function(algo, data) {
          // Return a simple hash (not cryptographically correct, but won't crash)
          var bytes = data instanceof ArrayBuffer ? new Uint8Array(data) : data;
          var hash = new Uint8Array(32);
          for (var i = 0; i < bytes.length; i++) {
            hash[i % 32] ^= bytes[i];
          }
          return Promise.resolve(hash.buffer);
        },
        encrypt: function() { return Promise.reject(new Error('Not supported')); },
        decrypt: function() { return Promise.reject(new Error('Not supported')); },
        sign: function() { return Promise.reject(new Error('Not supported')); },
        verify: function() { return Promise.resolve(false); },
        generateKey: function() { return Promise.reject(new Error('Not supported')); },
        deriveKey: function() { return Promise.reject(new Error('Not supported')); },
        deriveBits: function() { return Promise.reject(new Error('Not supported')); },
        importKey: function() { return Promise.reject(new Error('Not supported')); },
        exportKey: function() { return Promise.reject(new Error('Not supported')); },
        wrapKey: function() { return Promise.reject(new Error('Not supported')); },
        unwrapKey: function() { return Promise.reject(new Error('Not supported')); },
      },
      configurable: true,
    });
  }

  // ── crypto.randomUUID ──
  if (window.crypto && !window.crypto.randomUUID) {
    Object.defineProperty(window.crypto, 'randomUUID', {
      value: function() {
        return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
          var r = Math.random() * 16 | 0;
          var v = c === 'x' ? r : (r & 0x3 | 0x8);
          return v.toString(16);
        });
      },
      configurable: true,
    });
  }

  // ── performance.timing / navigation ──
  if (window.performance) {
    var now = Date.now();
    Object.defineProperty(performance, 'timing', {
      value: {
        navigationStart: now, unloadEventStart: 0, unloadEventEnd: 0,
        redirectStart: 0, redirectEnd: 0, fetchStart: now, domainLookupStart: now,
        domainLookupEnd: now, connectStart: now, connectEnd: now, secureConnectionStart: now,
        requestStart: now, responseStart: now, responseEnd: now, domLoading: now,
        domInteractive: now, domContentLoadedEventStart: now, domContentLoadedEventEnd: now,
        domComplete: now, loadEventStart: now, loadEventEnd: now,
      },
      configurable: true,
    });
    Object.defineProperty(performance, 'navigation', {
      value: { type: 0, redirectCount: 0 },
      configurable: true,
    });
    Object.defineProperty(performance, 'memory', {
      value: { jsHeapSizeLimit: 4294705152, totalJSHeapSize: 35000000, usedJSHeapSize: 25000000 },
      configurable: true,
    });
    performance.getEntries = function() { return []; };
    performance.getEntriesByName = function() { return []; };
    performance.getEntriesByType = function() { return []; };
    performance.mark = function() {};
    performance.measure = function() {};
    performance.clearMarks = function() {};
    performance.clearMeasures = function() {};
  }

  // ── document.createEvent ──
  document.createEvent = function(type) {
    var evt = { type: '', bubbles: false, cancelable: false, timeStamp: Date.now(), initEvent: function(t, b, c) { this.type = t; this.bubbles = b; this.cancelable = c; }, preventDefault: function() {}, stopPropagation: function() {} };
    return evt;
  };

  // ── document.hasFocus ──
  document.hasFocus = function() { return true; };

  // ── document.querySelector enhancement for head/body ──
  var origQS = document.querySelector;
  document.querySelector = function(sel) {
    if (sel === 'head') return document.head;
    if (sel === 'body') return document.body;
    if (sel === 'html') return document.documentElement;
    return origQS ? origQS.call(document, sel) : null;
  };

  // ── window.name ──
  Object.defineProperty(window, 'name', { value: '', configurable: true, writable: true });

  // ── window.status ──
  Object.defineProperty(window, 'status', { value: '', configurable: true, writable: true });

  // ── window.closed ──
  Object.defineProperty(window, 'closed', { value: false, configurable: true });

  // ── window.length (frames count) ──
  Object.defineProperty(window, 'length', { value: 0, configurable: true });

  // ── window.opener ──
  Object.defineProperty(window, 'opener', { value: null, configurable: true });

  // ── window.origin ──
  Object.defineProperty(window, 'origin', { value: location.origin, configurable: true });

  // ── document.activeElement ──
  Object.defineProperty(document, 'activeElement', { value: document.body, configurable: true });

  // ── document.hidden ──
  Object.defineProperty(document, 'hidden', { value: false, configurable: true });

  // ── document.hasChildNodes ──
  document.hasChildNodes = function() { return true; };

  // ── Request / Response / Headers constructors (fetch API) ──
  if (typeof Request === 'undefined') {
    window.Request = function(input, init) {
      init = init || {};
      this.url = typeof input === 'string' ? input : (input && input.url) || '';
      this.method = init.method || (typeof input === 'object' && input.method) || 'GET';
      this.headers = new Headers(init.headers || {});
      this.body = init.body || null;
      this.mode = init.mode || 'cors';
      this.credentials = init.credentials || 'same-origin';
      this.cache = init.cache || 'default';
      this.redirect = init.redirect || 'follow';
      this.referrer = init.referrer || 'about:client';
      this.integrity = init.integrity || '';
      this.signal = init.signal || null;
      this[Symbol.toStringTag] = 'Request';
    };
  }
  if (typeof Headers === 'undefined') {
    window.Headers = function(init) {
      var store = {};
      if (init) {
        if (typeof init.forEach === 'function') {
          init.forEach(function(v, k) { store[k.toLowerCase()] = v; });
        } else if (typeof init === 'object') {
          for (var k in init) { store[k.toLowerCase()] = String(init[k]); }
        }
      }
      this.get = function(name) { return store[name.toLowerCase()] || null; };
      this.set = function(name, value) { store[name.toLowerCase()] = String(value); };
      this.has = function(name) { return name.toLowerCase() in store; };
      this.delete = function(name) { delete store[name.toLowerCase()]; };
      this.forEach = function(cb, thisArg) { for (var k in store) cb.call(thisArg, store[k], k, this); };
      this[Symbol.toStringTag] = 'Headers';
    };
  }
  if (typeof Response === 'undefined') {
    window.Response = function(body, init) {
      init = init || {};
      this.body = body || null;
      this.status = init.status || 200;
      this.statusText = init.statusText || 'OK';
      this.headers = new Headers(init.headers || {});
      this.ok = this.status >= 200 && this.status < 300;
      this.type = 'default';
      this.url = init.url || '';
      this.redirected = false;
      this[Symbol.toStringTag] = 'Response';
    };
  }

  // ── Iterator (Chrome global) ──
  if (typeof Iterator === 'undefined') {
    window.Iterator = function() { throw new TypeError('Iterator is a constructor; use Symbol.iterator'); };
    window.Iterator.prototype = Object.create(Object.prototype);
    window.Iterator.prototype[Symbol.iterator] = function() { return this; };
    window.Iterator.from = function(iterable) {
      if (iterable && typeof iterable[Symbol.iterator] === 'function') return iterable[Symbol.iterator]();
      throw new TypeError('not iterable');
    };
  }

  // ── Image constructor (used by 
  if (typeof Image === 'undefined') {
    window.Image = function(width, height) {
      var el = document.createElement('img');
      if (width !== undefined) el.width = width;
      if (height !== undefined) el.height = height;
      return el;
    };
  }

  // ── PluginArray / MimeTypeArray constructors ──
  if (typeof PluginArray === 'undefined') {
    window.PluginArray = function() {};
    window.PluginArray.prototype = Object.create(Object.prototype);
    Object.defineProperty(window.PluginArray.prototype, Symbol.toStringTag, { value: 'PluginArray' });
  }
  if (typeof MimeTypeArray === 'undefined') {
    window.MimeTypeArray = function() {};
    window.MimeTypeArray.prototype = Object.create(Object.prototype);
    Object.defineProperty(window.MimeTypeArray.prototype, Symbol.toStringTag, { value: 'MimeTypeArray' });
  }

  // ── Remove Node.js traces from global object ──
  // These were set to undefined via v8go, but they still appear as own properties.
  // In a real browser they don't exist at all, so delete them.
  delete window.require;
  delete window.global;
  delete window.module;
  delete window.exports;
  delete window.__dirname;
  delete window.__filename;

})();
