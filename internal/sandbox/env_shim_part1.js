// env_shim.js — Post-context browser API enhancement
// Injected after v8go context creation to add APIs that can't be
// set via ObjectTemplate (complex objects, constructors, etc.)

(function() {
  'use strict';

  // ── navigator.plugins / mimeTypes ──
  var pluginData = [
    { name: 'PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
    { name: 'Chrome PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
    { name: 'Chromium PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
    { name: 'Microsoft Edge PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
    { name: 'WebKit built-in PDF', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
  ];
  var mimeData = [
    { type: 'application/pdf', suffixes: 'pdf', description: 'Portable Document Format' },
    { type: 'text/pdf', suffixes: 'pdf', description: 'Portable Document Format' },
  ];
  function makePluginArray() {
    var arr = [];
    for (var i = 0; i < pluginData.length; i++) {
      var p = pluginData[i];
      var plugin = { name: p.name, filename: p.filename, description: p.description, length: mimeData.length };
      for (var j = 0; j < mimeData.length; j++) {
        plugin[j] = { type: mimeData[j].type, suffixes: mimeData[j].suffixes, description: mimeData[j].description, enabledPlugin: plugin };
      }
      plugin.item = function(idx) { return this[idx] || null; };
      plugin.namedItem = function(name) { return null; };
      arr[i] = plugin;
    }
    arr.item = function(idx) { return this[idx] || null; };
    arr.namedItem = function(name) { return null; };
    arr.refresh = function() {};
    Object.defineProperty(arr, 'length', { value: pluginData.length });
    return arr;
  }
  Object.defineProperty(navigator, 'plugins', { value: makePluginArray(), configurable: true });
  Object.defineProperty(navigator, 'mimeTypes', {
    value: (function() {
      var arr = [];
      for (var i = 0; i < mimeData.length; i++) {
        arr[i] = { type: mimeData[i].type, suffixes: mimeData[i].suffixes, description: mimeData[i].description };
      }
      arr.item = function(idx) { return this[idx] || null; };
      arr.namedItem = function(name) { return null; };
      Object.defineProperty(arr, 'length', { value: mimeData.length });
      return arr;
    })(),
    configurable: true
  });

  // ── navigator.mediaDevices ──
  Object.defineProperty(navigator, 'mediaDevices', {
    value: {
      enumerateDevices: function() {
        return Promise.resolve([
          { kind: 'audioinput', deviceId: 'default', groupId: 'group1', label: '' },
          { kind: 'audiooutput', deviceId: 'default', groupId: 'group1', label: '' },
          { kind: 'videoinput', deviceId: 'default', groupId: 'group2', label: '' },
        ]);
      },
      getUserMedia: function() { return Promise.reject(new Error('Permission denied')); },
      getDisplayMedia: function() { return Promise.reject(new Error('Permission denied')); },
      getSupportedConstraints: function() {
        return { aspectRatio: true, autoGainControl: true, channelCount: true, deviceId: true,
                 echoCancellation: true, facingMode: true, frameRate: true, height: true,
                 noiseSuppression: true, sampleRate: true, sampleSize: true, volume: true, width: true };
      },
      ondevicechange: null,
      addEventListener: function() {}, removeEventListener: function() {},
    },
    configurable: true
  });

  // ── navigator.serviceWorker ──
  Object.defineProperty(navigator, 'serviceWorker', {
    value: {
      controller: null,
      ready: Promise.resolve({ active: null }),
      register: function() { return Promise.reject(new Error('Not supported')); },
      getRegistrations: function() { return Promise.resolve([]); },
      onmessage: null,
      addEventListener: function() {}, removeEventListener: function() {},
    },
    configurable: true
  });

  // ── navigator.clipboard ──
  Object.defineProperty(navigator, 'clipboard', {
    value: {
      readText: function() { return Promise.resolve(''); },
      writeText: function(text) { return Promise.resolve(); },
      read: function() { return Promise.resolve([]); },
      write: function() { return Promise.resolve(); },
    },
    configurable: true
  });

  // ── navigator.geolocation ──
  Object.defineProperty(navigator, 'geolocation', {
    value: {
      getCurrentPosition: function(success, error) {
        if (error) error({ code: 1, message: 'Permission denied' });
      },
      watchPosition: function() { return 0; },
      clearWatch: function() {},
    },
    configurable: true
  });

  // ── navigator.bluetooth ──
  Object.defineProperty(navigator, 'bluetooth', {
    value: { getAvailability: function() { return Promise.resolve(false); }, requestDevice: function() { return Promise.reject(new Error('Not supported')); } },
    configurable: true
  });

  // ── navigator.usb ──
  Object.defineProperty(navigator, 'usb', {
    value: { getDevices: function() { return Promise.resolve([]); }, requestDevice: function() { return Promise.reject(new Error('Not supported')); } },
    configurable: true
  });

  // ── navigator.credentials ──
  Object.defineProperty(navigator, 'credentials', {
    value: { get: function() { return Promise.resolve(null); }, store: function() { return Promise.resolve(); }, create: function() { return Promise.resolve(null); }, preventSilentAccess: function() { return Promise.resolve(); } },
    configurable: true
  });

  // ── navigator.scheduling ──
  Object.defineProperty(navigator, 'scheduling', {
    value: { isInputPending: function() { return false; } },
    configurable: true
  });

  // ── navigator.storage ──
  Object.defineProperty(navigator, 'storage', {
    value: { estimate: function() { return Promise.resolve({ quota: 296352849920, usage: 0 }); }, persist: function() { return Promise.resolve(false); }, persisted: function() { return Promise.resolve(false); } },
    configurable: true
  });

  // ── navigator.locks ──
  Object.defineProperty(navigator, 'locks', {
    value: { request: function() { return Promise.resolve(); }, query: function() { return Promise.resolve({ held: [], pending: [] }); } },
    configurable: true
  });

  // ── navigator.wakeLock ──
  Object.defineProperty(navigator, 'wakeLock', {
    value: { request: function() { return Promise.reject(new Error('Not supported')); } },
    configurable: true
  });

  // ── navigator.ink ──
  Object.defineProperty(navigator, 'ink', {
    value: { requestPresenter: function() { return Promise.resolve({}); } },
    configurable: true
  });

  // ── navigator.presentation ──
  Object.defineProperty(navigator, 'presentation', {
    value: { defaultRequest: null, receiver: null },
    configurable: true
  });

})();
