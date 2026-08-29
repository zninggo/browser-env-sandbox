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
          // Real SHA-256 via Go callback __besSha256 (injectCryptoHelpers).
          // Go side accepts string; JS converts ArrayBuffer/TypedArray → string
          // (latin1: byte value == charCodeAt) before calling.
          try {
            var input;
            if (typeof data === 'string') {
              input = data;
            } else if (data instanceof ArrayBuffer) {
              var u8 = new Uint8Array(data);
              input = '';
              for (var i = 0; i < u8.length; i++) input += String.fromCharCode(u8[i]);
            } else if (data && typeof data.length === 'number') {
              // TypedArray
              input = '';
              for (var i = 0; i < data.length; i++) input += String.fromCharCode(data[i]);
            } else {
              input = String(data);
            }
            var hexStr = __besSha256(input);
            // Convert hex → ArrayBuffer (32 bytes)
            var buf = new ArrayBuffer(32);
            var view = new Uint8Array(buf);
            for (var i = 0; i < 32; i++) {
              view[i] = parseInt(hexStr.substr(i * 2, 2), 16);
            }
            return Promise.resolve(buf);
          } catch(e) {
            return Promise.reject(e);
          }
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

  // ════════════════════════════════════════════════════════════════
  // 通用补环境增强（DTraitSDK / challenge-template / bdms 依赖）
  // ════════════════════════════════════════════════════════════════

  // ── TypedArray iterator polyfill ──
  // DTraitSDK (dtrait-core.js 模块 8254) 依赖 Uint8Array.prototype[Symbol.iterator]，
  // 部分 V8 构建中 TypedArray prototype 缺 values/keys/entries/Symbol.iterator，
  // 导致 "Cannot convert undefined or null to object"。
  // 只在缺失时补，不覆盖 V8 原生实现。
  var _taCtors = ['Int8Array','Uint8Array','Uint8ClampedArray','Int16Array','Uint16Array','Int32Array','Uint32Array','Float32Array','Float64Array','BigInt64Array','BigUint64Array'];
  _taCtors.forEach(function(name) {
    var Ctor = window[name];
    if (!Ctor || !Ctor.prototype) return;
    var proto = Ctor.prototype;
    if (typeof proto.values !== 'function') {
      proto.values = function() {
        var i = 0, arr = this;
        return { next: function() { return i < arr.length ? { value: arr[i++], done: false } : { value: undefined, done: true }; } };
      };
    }
    if (typeof proto.keys !== 'function') {
      proto.keys = function() {
        var i = 0, arr = this;
        return { next: function() { return i < arr.length ? { value: i++, done: false } : { value: undefined, done: true }; } };
      };
    }
    if (typeof proto.entries !== 'function') {
      proto.entries = function() {
        var i = 0, arr = this;
        return { next: function() { return i < arr.length ? { value: [i, arr[i++]], done: false } : { value: undefined, done: true }; } };
      };
    }
    if (typeof proto[Symbol.iterator] !== 'function') {
      proto[Symbol.iterator] = proto.values;
    }
  });

  // ── crypto.getRandomValues: 真正填充随机字节 ──
  // Go 侧的 injectCrypto 原样返回输入（不填字节），SDK 生成 nonce/IV 时拿到全 0。
  // 用 Math.random 填充——非密码学安全，但满足 SDK 格式需求且不崩。
  if (window.crypto && typeof crypto.getRandomValues === 'function') {
    var _origGetRandomValues = crypto.getRandomValues.bind(crypto);
    crypto.getRandomValues = function(arr) {
      if (!arr || typeof arr.length !== 'number') {
        throw new TypeError('getRandomValues: argument is not a TypedArray');
      }
      for (var i = 0; i < arr.length; i++) {
        arr[i] = Math.floor(Math.random() * 256);
      }
      return arr;
    };
    try { nativeFns.add(crypto.getRandomValues); } catch(e) {}
  }

  // ── localStorage / sessionStorage: 完整 Storage API ──
  // Go 侧 injectStorage 只设空 ObjectTemplate，无 getItem/setItem。
  // SDK 存储会话状态（sdk_source_info / bit_env 等）需要完整接口。
  function _besMakeStorage() {
    var store = Object.create(null);
    return {
      get length() { return Object.keys(store).length; },
      key: function(i) { var keys = Object.keys(store); return i >= 0 && i < keys.length ? keys[i] : null; },
      getItem: function(name) { return Object.prototype.hasOwnProperty.call(store, String(name)) ? store[String(name)] : null; },
      setItem: function(name, value) { store[String(name)] = String(value); },
      removeItem: function(name) { delete store[String(name)]; },
      clear: function() { store = Object.create(null); },
    };
  }
  try { Object.defineProperty(window, 'localStorage', { value: _besMakeStorage(), configurable: true }); } catch(e) {}
  try { Object.defineProperty(window, 'sessionStorage', { value: _besMakeStorage(), configurable: true }); } catch(e) {}

  // ── iframe.contentWindow / contentDocument ──
  // challenge-template.js 的环境采集器在 iframe.contentWindow 里跑（拿"干净"环境），
  // createElement('iframe') 返回的 stub 无 contentWindow → 直接崩。
  // contentWindow 指回 window（globalThis），contentDocument 指回 document。
  var _origCE_forIframe = document.createElement;
  document.createElement = function(tagName) {
    var el = _origCE_forIframe.call(document, tagName);
    if (!el) return el;
    tagName = String(tagName).toLowerCase();
    if (tagName === 'iframe') {
      Object.defineProperty(el, 'contentWindow', { value: window, configurable: true });
      Object.defineProperty(el, 'contentDocument', { value: document, configurable: true });
      try { nativeFns.add; } catch(e) {}
    }
    return el;
  };

  // ── performance.timeOrigin + now() 真实递增值 ──
  // Go 侧 injectPerformance 的 now() 恒返回 0，无 timeOrigin。
  // 指纹采集器用 performance.now() 做时间测量，恒 0 异常。
  if (window.performance) {
    var _perfOrigin = (typeof performance.timeOrigin === 'number') ? performance.timeOrigin : (Date.now() - 1000);
    if (typeof performance.timeOrigin !== 'number') {
      try { Object.defineProperty(performance, 'timeOrigin', { value: _perfOrigin, configurable: true }); } catch(e) {}
    }
    performance.now = function() { return Date.now() - _perfOrigin; };
    try { nativeFns.add(performance.now); } catch(e) {}
  }

  // ── MessageChannel: real port1↔port2 messaging (Bug 9 fix) ──
  // Go-side stub uses same port object for port1/port2 with noop postMessage.
  // Override with independent ports where postMessage triggers onmessage on
  // the opposite port (via setTimeout(0) for async delivery).
  window.MessageChannel = function() {
    var port1 = { onmessage: null, _other: null, closed: false };
    var port2 = { onmessage: null, _other: null, closed: false };
    port1._other = port2;
    port2._other = port1;
    port1.postMessage = function(data) {
      if (this.closed || !this._other || this._other.closed) return;
      var target = this._other;
      var msg = { data: data };
      setTimeout(function() {
        if (target.onmessage) target.onmessage(msg);
      }, 0);
    };
    port2.postMessage = function(data) {
      if (this.closed || !this._other || this._other.closed) return;
      var target = this._other;
      var msg = { data: data };
      setTimeout(function() {
        if (target.onmessage) target.onmessage(msg);
      }, 0);
    };
    port1.close = function() { this.closed = true; };
    port2.close = function() { this.closed = true; };
    port1.start = function() {};
    port2.start = function() {};
    this.port1 = port1;
    this.port2 = port2;
  };

  // ── Canvas 2D getImageData/createImageData fix (Bug 7) ──
  // Go callback can't create JS objects (v8go NewValue only accepts primitives).
  // Override getContext to wrap the 2d context with JS-side getImageData/createImageData.
  try {
    var _origCE_canvas = document.createElement;
    document.createElement = function(tagName) {
      var el = _origCE_canvas.call(document, tagName);
      if (el && typeof tagName === 'string' && tagName.toLowerCase() === 'canvas') {
        var _origGC = el.getContext;
        el.getContext = function(type) {
          var ctx = _origGC.call(this, type);
          if (ctx && type === '2d') {
            // Bug 7 fix: real getImageData with proper Uint8ClampedArray
            ctx.getImageData = function(x, y, w, h) {
              w = w || 1; h = h || 1;
              if (w < 1) w = 1; if (h < 1) h = 1;
              return {
                data: new Uint8ClampedArray(w * h * 4),
                width: w,
                height: h,
              };
            };
            ctx.createImageData = function(w, h) {
              if (arguments.length === 1) { h = w; }
              w = w || 1; h = h || 1;
              if (w < 1) w = 1; if (h < 1) h = 1;
              return {
                data: new Uint8ClampedArray(w * h * 4),
                width: w,
                height: h,
              };
            };
          }
          return ctx;
        };
      }
      return el;
    };
  } catch(e) {}

})();
