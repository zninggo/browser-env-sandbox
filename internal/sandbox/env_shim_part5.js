// env_shim_part5.js — Comprehensive global constructor stubs
// Auto-generated: covers all Chrome global constructors missing from sandbox.
// Each stub is a function with prototype + Symbol.toStringTag so that
// typeof X === 'function', X.prototype.constructor === X, and
// Object.prototype.toString.call(new X()) === '[object X]'.

(function() {
  'use strict';

  // Special constructors with functional stubs

  // --- AudioContext / OfflineAudioContext ---
  if (typeof AudioContext === 'undefined') {
    window.AudioContext = function() {
      this.sampleRate = 44100;
      this.state = 'running';
      this.destination = { channelCount: 2, maxChannelCount: 2 };
      this.currentTime = 0;
      this.createOscillator = function() { return { type: 'sine', frequency: { value: 440 }, connect: function(){}, start: function(){}, stop: function(){} }; };
      this.createAnalyser = function() { return { connect: function(){}, getByteFrequencyData: function(a){}, getFloatFrequencyData: function(a){} }; };
      this.createGain = function() { return { gain: { value: 1 }, connect: function(){} }; };
      this.createScriptProcessor = function() { return { connect: function(){}, onaudioprocess: null }; };
      this.close = function() { return Promise.resolve(); };
      this[Symbol.toStringTag] = 'AudioContext';
    };
  }
  if (typeof OfflineAudioContext === 'undefined') {
    window.OfflineAudioContext = function(channels, length, sampleRate) {
      this.length = length || 1;
      this.sampleRate = sampleRate || 44100;
      this[Symbol.toStringTag] = 'OfflineAudioContext';
    };
    window.OfflineAudioContext.prototype = Object.create(Object.prototype);
    if (typeof AudioContext !== 'undefined') Object.setPrototypeOf(window.OfflineAudioContext.prototype, window.AudioContext.prototype);
  }

  // --- Screen ---
  if (typeof Screen === 'undefined') {
    window.Screen = function() { this[Symbol.toStringTag] = 'Screen'; };
  }

  // --- History ---
  if (typeof History === 'undefined') {
    window.History = function() { this[Symbol.toStringTag] = 'History'; };
  }

  // --- Location ---
  if (typeof Location === 'undefined') {
    window.Location = function() { this[Symbol.toStringTag] = 'Location'; };
  }

  // --- Navigator ---
  if (typeof Navigator === 'undefined') {
    window.Navigator = function() { this[Symbol.toStringTag] = 'Navigator'; };
  }

  // --- Storage ---
  if (typeof Storage === 'undefined') {
    window.Storage = function() { this[Symbol.toStringTag] = 'Storage'; };
  }

  // --- Crypto / SubtleCrypto ---
  if (typeof Crypto === 'undefined') {
    window.Crypto = function() { this[Symbol.toStringTag] = 'Crypto'; };
  }
  if (typeof SubtleCrypto === 'undefined') {
    window.SubtleCrypto = function() { this[Symbol.toStringTag] = 'SubtleCrypto'; };
  }

  // --- Performance ---
  if (typeof Performance === 'undefined') {
    window.Performance = function() { this[Symbol.toStringTag] = 'Performance'; };
  }

  // --- Document / Element / Node ---
  if (typeof Document === 'undefined') {
    window.Document = function() { this[Symbol.toStringTag] = 'Document'; };
  }
  if (typeof Element === 'undefined') {
    window.Element = function() { this[Symbol.toStringTag] = 'Element'; };
  }
  if (typeof Node === 'undefined') {
    window.Node = function() { this[Symbol.toStringTag] = 'Node'; };
    window.Node.ELEMENT_NODE = 1; window.Node.TEXT_NODE = 3; window.Node.DOCUMENT_NODE = 9;
  }
  if (typeof NodeList === 'undefined') {
    window.NodeList = function() { this[Symbol.toStringTag] = 'NodeList'; };
  }
  if (typeof HTMLCollection === 'undefined') {
    window.HTMLCollection = function() { this[Symbol.toStringTag] = 'HTMLCollection'; };
  }

  // --- File / FileReader ---
  if (typeof File === 'undefined') {
    window.File = function(bits, name, opts) { this.name = name || ''; this.size = 0; this.type = (opts && opts.type) || ''; this[Symbol.toStringTag] = 'File'; };
  }
  if (typeof FileReader === 'undefined') {
    window.FileReader = function() { this.readyState = 0; this.result = null; this.onload = null; this.onerror = null; this[Symbol.toStringTag] = 'FileReader'; };
    window.FileReader.EMPTY = 0; window.FileReader.LOADING = 1; window.FileReader.DONE = 2;
    window.FileReader.prototype.readAsArrayBuffer = function(blob) {
      var self = this;
      self.readyState = 1;
      setTimeout(function() {
        try {
          var bytes = blob && blob._besBytes ? blob._besBytes() : new Uint8Array(0);
          var ab = new ArrayBuffer(bytes.length);
          new Uint8Array(ab).set(bytes);
          self.result = ab;
          self.readyState = 2;
          if (self.onload) self.onload({ target: self });
        } catch(e) { self.readyState = 2; if (self.onerror) self.onerror(e); }
      }, 0);
    };
    window.FileReader.prototype.readAsText = function(blob, encoding) {
      var self = this;
      self.readyState = 1;
      setTimeout(function() {
        try {
          var bytes = blob && blob._besBytes ? blob._besBytes() : new Uint8Array(0);
          self.result = __besBytesToUTF8 ? __besBytesToUTF8(bytes) : '';
          for (var i = 0, s = ''; i < bytes.length; i++) s += String.fromCharCode(bytes[i]);
          if (!self.result) self.result = s;
          self.readyState = 2;
          if (self.onload) self.onload({ target: self });
        } catch(e) { self.readyState = 2; if (self.onerror) self.onerror(e); }
      }, 0);
    };
    window.FileReader.prototype.readAsDataURL = function(blob) {
      var self = this;
      self.readyState = 1;
      setTimeout(function() {
        try {
          var bytes = blob && blob._besBytes ? blob._besBytes() : new Uint8Array(0);
          var b64 = btoa(String.fromCharCode.apply(null, bytes));
          self.result = 'data:' + (blob && blob.type || 'application/octet-stream') + ';base64,' + b64;
          self.readyState = 2;
          if (self.onload) self.onload({ target: self });
        } catch(e) { self.readyState = 2; if (self.onerror) self.onerror(e); }
      }, 0);
    };
    window.FileReader.prototype.abort = function() { this.readyState = 2; };
  }
  if (typeof FileList === 'undefined') {
    window.FileList = function() { this[Symbol.toStringTag] = 'FileList'; };
  }

  // --- Canvas ---
  if (typeof CanvasRenderingContext2D === 'undefined') {
    window.CanvasRenderingContext2D = function() { this[Symbol.toStringTag] = 'CanvasRenderingContext2D'; };
  }
  if (typeof WebGLRenderingContext === 'undefined') {
    window.WebGLRenderingContext = function() { this[Symbol.toStringTag] = 'WebGLRenderingContext'; };
  }
  if (typeof WebGL2RenderingContext === 'undefined') {
    window.WebGL2RenderingContext = function() { this[Symbol.toStringTag] = 'WebGL2RenderingContext'; };
  }
  if (typeof ImageData === 'undefined') {
    window.ImageData = function(data, w, h) { this.data = data || []; this.width = w || 0; this.height = h || 0; this[Symbol.toStringTag] = 'ImageData'; };
  }
  if (typeof CanvasGradient === 'undefined') {
    window.CanvasGradient = function() { this[Symbol.toStringTag] = 'CanvasGradient'; };
  }
  if (typeof CanvasPattern === 'undefined') {
    window.CanvasPattern = function() { this[Symbol.toStringTag] = 'CanvasPattern'; };
  }
  if (typeof Path2D === 'undefined') {
    window.Path2D = function() { this[Symbol.toStringTag] = 'Path2D'; };
  }

  // --- Streams ---
  if (typeof ReadableStream === 'undefined') {
    window.ReadableStream = function() { this[Symbol.toStringTag] = 'ReadableStream'; };
  }
  if (typeof WritableStream === 'undefined') {
    window.WritableStream = function() { this[Symbol.toStringTag] = 'WritableStream'; };
  }
  if (typeof TransformStream === 'undefined') {
    window.TransformStream = function() { this[Symbol.toStringTag] = 'TransformStream'; };
  }
  if (typeof CompressionStream === 'undefined') {
    window.CompressionStream = function() { this[Symbol.toStringTag] = 'CompressionStream'; };
  }
  if (typeof DecompressionStream === 'undefined') {
    window.DecompressionStream = function() { this[Symbol.toStringTag] = 'DecompressionStream'; };
  }
  if (typeof TextDecoderStream === 'undefined') {
    window.TextDecoderStream = function() { this[Symbol.toStringTag] = 'TextDecoderStream'; };
  }
  if (typeof TextEncoderStream === 'undefined') {
    window.TextEncoderStream = function() { this[Symbol.toStringTag] = 'TextEncoderStream'; };
  }
  if (typeof ByteLengthQueuingStrategy === 'undefined') {
    window.ByteLengthQueuingStrategy = function() { this[Symbol.toStringTag] = 'ByteLengthQueuingStrategy'; };
  }
  if (typeof CountQueuingStrategy === 'undefined') {
    window.CountQueuingStrategy = function() { this[Symbol.toStringTag] = 'CountQueuingStrategy'; };
  }

  // --- BroadcastChannel ---
  if (typeof BroadcastChannel === 'undefined') {
    window.BroadcastChannel = function(name) { this.name = name || ''; this[Symbol.toStringTag] = 'BroadcastChannel'; };
  }

  // --- URLPattern ---
  if (typeof URLPattern === 'undefined') {
    window.URLPattern = function(input) { this[Symbol.toStringTag] = 'URLPattern'; };
  }

  // --- AbortSignal ---
  if (typeof AbortSignal === 'undefined') {
    window.AbortSignal = function() { this.aborted = false; this[Symbol.toStringTag] = 'AbortSignal'; };
  }

  // --- DOMException (if not already defined) ---
  if (typeof DOMException === 'undefined') {
    window.DOMException = function(message, name) { this.message = message || ''; this.name = name || 'Error'; this[Symbol.toStringTag] = 'DOMException'; };
    window.DOMException.prototype = Object.create(Error.prototype);
  }

  // --- DOMRect / DOMPoint / DOMMatrix ---
  if (typeof DOMRect === 'undefined') {
    window.DOMRect = function(x,y,w,h) { this.x=x||0;this.y=y||0;this.width=w||0;this.height=h||0;this.top=y||0;this.bottom=(y||0)+(h||0);this.left=x||0;this.right=(x||0)+(w||0);this[Symbol.toStringTag]='DOMRect'; };
  }
  if (typeof DOMPoint === 'undefined') {
    window.DOMPoint = function(x,y,z,w) { this.x=x||0;this.y=y||0;this.z=z||0;this.w=w||1;this[Symbol.toStringTag]='DOMPoint'; };
  }
  if (typeof DOMMatrix === 'undefined') {
    window.DOMMatrix = function() { this[Symbol.toStringTag]='DOMMatrix'; };
  }

  // ── Event.prototype methods (Bug 43 residual fix) ──
  // Go 侧模板构造的 new Event() 实例不继承 window.Event.prototype（v8go 模板
  // 实例的 [[Prototype]] 是 Object.prototype），原型方法对它们无效。所以：
  // 1) 用 Go 构造器造实例 → setPrototypeOf 挂上 JS Event.prototype（带活方法）
  // 2) 用这个包装替换 window.Event，后续 new Event() 全部走 JS 路径
  // 3) CustomEvent/MessageEvent 等继承 Event.prototype，方法一并生效
  if (typeof Event !== 'undefined' && window.Event && typeof window.Event === 'function') {
    var _besGoEvent = window.Event;
    var _evtMethods = {
      preventDefault: function() { if (this.cancelable) this.defaultPrevented = true; },
      stopPropagation: function() { this.cancelBubble = true; },
      stopImmediatePropagation: function() { this.cancelBubble = true; this._besStoppedImmediate = true; },
      initEvent: function(type, bubbles, cancelable) {
        this.type = type;
        this.bubbles = !!bubbles;
        this.cancelable = !!cancelable;
      },
    };
    function BesEvent(type, init) {
      var inst = _besGoEvent(type);
      init = init || {};
      if (typeof inst.bubbles !== 'undefined') inst.bubbles = !!init.bubbles;
      if (typeof inst.cancelable !== 'undefined') inst.cancelable = !!init.cancelable;
      if (init.composed !== undefined && typeof inst.composed !== 'undefined') inst.composed = !!init.composed;
      // Go 模板在实例上挂的 noop 方法会遮蔽原型上的活方法（实例自有属性优先），
      // 删掉它们，让 BesEvent.prototype 的带状态实现生效。
      delete inst.preventDefault;
      delete inst.stopPropagation;
      delete inst.stopImmediatePropagation;
      delete inst.initEvent;
      Object.setPrototypeOf(inst, BesEvent.prototype);
      return inst;
    }
    BesEvent.prototype = Object.create(Object.prototype);
    for (var m in _evtMethods) {
      Object.defineProperty(BesEvent.prototype, m, { value: _evtMethods[m], writable: true, configurable: true });
    }
    Object.defineProperty(BesEvent.prototype, 'constructor', { value: BesEvent, writable: true, configurable: true });
    // writable:true — 子类构造器（CustomEvent 等）会给实例赋 this[Symbol.toStringTag]，
    // 若原型 tag 不可写，赋值会沿原型链失败抛 TypeError。
    try { Object.defineProperty(BesEvent.prototype, Symbol.toStringTag, { value: 'Event', writable: true, configurable: true }); } catch(e) {}
    Object.setPrototypeOf(BesEvent, _besGoEvent); // static props (Event.CAPTURING_PHASE 等) 透传
    window.Event = BesEvent;
    try { nativeFns.add(window.Event); } catch(e) {}
  }

  // --- CustomEvent ---
  if (typeof CustomEvent === 'undefined') {
    window.CustomEvent = function(type, opts) { this.type = type; this.detail = opts && opts.detail || null; this[Symbol.toStringTag] = 'CustomEvent'; };
    window.CustomEvent.prototype = Object.create(window.Event.prototype);
  }

  // --- MessageEvent / ProgressEvent / ErrorEvent ---
  if (typeof MessageEvent === 'undefined') {
    window.MessageEvent = function(type, opts) { this.type = type; this.data = opts && opts.data || null; this[Symbol.toStringTag] = 'MessageEvent'; };
    window.MessageEvent.prototype = Object.create(window.Event.prototype);
  }
  if (typeof ProgressEvent === 'undefined') {
    window.ProgressEvent = function(type, opts) { this.type = type; this.loaded = opts && opts.loaded || 0; this.total = opts && opts.total || 0; this[Symbol.toStringTag] = 'ProgressEvent'; };
    window.ProgressEvent.prototype = Object.create(window.Event.prototype);
  }
  if (typeof ErrorEvent === 'undefined') {
    window.ErrorEvent = function(type, opts) { this.type = type; this.message = opts && opts.message || ''; this[Symbol.toStringTag] = 'ErrorEvent'; };
    window.ErrorEvent.prototype = Object.create(window.Event.prototype);
  }

  // --- FontFace ---
  if (typeof FontFace === 'undefined') {
    window.FontFace = function(family, source) { this.family = family || ''; this[Symbol.toStringTag] = 'FontFace'; };
  }

  // --- MessagePort ---
  if (typeof MessagePort === 'undefined') {
    window.MessagePort = function() { this[Symbol.toStringTag] = 'MessagePort'; };
  }

  // --- SharedWorker ---
  if (typeof SharedWorker === 'undefined') {
    window.SharedWorker = function() { this[Symbol.toStringTag] = 'SharedWorker'; };
  }

  // --- EventSource ---
  if (typeof EventSource === 'undefined') {
    window.EventSource = function(url) { this.url = url; this.readyState = 0; this[Symbol.toStringTag] = 'EventSource'; };
  }

  // --- PromiseRejectionEvent ---
  if (typeof PromiseRejectionEvent === 'undefined') {
    window.PromiseRejectionEvent = function(type, opts) { this.type = type; this.reason = opts && opts.reason; this[Symbol.toStringTag] = 'PromiseRejectionEvent'; };
    window.PromiseRejectionEvent.prototype = Object.create(window.Event.prototype);
  }

  // --- All remaining constructors: generic stubs ---
  // Each gets: typeof X === 'function', X.prototype.constructor === X,
  // Object.prototype.toString.call(new X()) === '[object X]'

  var _defined = Object.getOwnPropertyNames(window);
  var _definedSet = {};
  for (var _i = 0; _i < _defined.length; _i++) _definedSet[_defined[_i]] = true;

  // 813 generic stubs
  if (typeof AbsoluteOrientationSensor === 'undefined') { window.AbsoluteOrientationSensor = function() { this[Symbol.toStringTag] = 'AbsoluteOrientationSensor'; }; }
  if (typeof AbstractRange === 'undefined') { window.AbstractRange = function() { this[Symbol.toStringTag] = 'AbstractRange'; }; }
  if (typeof Accelerometer === 'undefined') { window.Accelerometer = function() { this[Symbol.toStringTag] = 'Accelerometer'; }; }
  if (typeof AnalyserNode === 'undefined') { window.AnalyserNode = function() { this[Symbol.toStringTag] = 'AnalyserNode'; }; }
  if (typeof Animation === 'undefined') { window.Animation = function() { this[Symbol.toStringTag] = 'Animation'; }; }
  if (typeof AnimationEffect === 'undefined') { window.AnimationEffect = function() { this[Symbol.toStringTag] = 'AnimationEffect'; }; }
  if (typeof AnimationEvent === 'undefined') { window.AnimationEvent = function() { this[Symbol.toStringTag] = 'AnimationEvent'; }; }
  if (typeof AnimationPlaybackEvent === 'undefined') { window.AnimationPlaybackEvent = function() { this[Symbol.toStringTag] = 'AnimationPlaybackEvent'; }; }
  if (typeof AnimationTimeline === 'undefined') { window.AnimationTimeline = function() { this[Symbol.toStringTag] = 'AnimationTimeline'; }; }
  if (typeof AnimationTrigger === 'undefined') { window.AnimationTrigger = function() { this[Symbol.toStringTag] = 'AnimationTrigger'; }; }
  if (typeof AsyncDisposableStack === 'undefined') { window.AsyncDisposableStack = function() { this[Symbol.toStringTag] = 'AsyncDisposableStack'; }; }
  if (typeof Attr === 'undefined') { window.Attr = function() { this[Symbol.toStringTag] = 'Attr'; }; }
  if (typeof AudioBuffer === 'undefined') { window.AudioBuffer = function() { this[Symbol.toStringTag] = 'AudioBuffer'; }; }
  if (typeof AudioBufferSourceNode === 'undefined') { window.AudioBufferSourceNode = function() { this[Symbol.toStringTag] = 'AudioBufferSourceNode'; }; }
  if (typeof AudioData === 'undefined') { window.AudioData = function() { this[Symbol.toStringTag] = 'AudioData'; }; }
  if (typeof AudioDecoder === 'undefined') { window.AudioDecoder = function() { this[Symbol.toStringTag] = 'AudioDecoder'; }; }
  if (typeof AudioDestinationNode === 'undefined') { window.AudioDestinationNode = function() { this[Symbol.toStringTag] = 'AudioDestinationNode'; }; }
  if (typeof AudioEncoder === 'undefined') { window.AudioEncoder = function() { this[Symbol.toStringTag] = 'AudioEncoder'; }; }
  if (typeof AudioListener === 'undefined') { window.AudioListener = function() { this[Symbol.toStringTag] = 'AudioListener'; }; }
  if (typeof AudioNode === 'undefined') { window.AudioNode = function() { this[Symbol.toStringTag] = 'AudioNode'; }; }
  if (typeof AudioParam === 'undefined') { window.AudioParam = function() { this[Symbol.toStringTag] = 'AudioParam'; }; }
  if (typeof AudioParamMap === 'undefined') { window.AudioParamMap = function() { this[Symbol.toStringTag] = 'AudioParamMap'; }; }
  if (typeof AudioPlaybackStats === 'undefined') { window.AudioPlaybackStats = function() { this[Symbol.toStringTag] = 'AudioPlaybackStats'; }; }
  if (typeof AudioProcessingEvent === 'undefined') { window.AudioProcessingEvent = function() { this[Symbol.toStringTag] = 'AudioProcessingEvent'; }; }
  if (typeof AudioScheduledSourceNode === 'undefined') { window.AudioScheduledSourceNode = function() { this[Symbol.toStringTag] = 'AudioScheduledSourceNode'; }; }
  if (typeof AudioSinkInfo === 'undefined') { window.AudioSinkInfo = function() { this[Symbol.toStringTag] = 'AudioSinkInfo'; }; }
  if (typeof AudioWorklet === 'undefined') { window.AudioWorklet = function() { this[Symbol.toStringTag] = 'AudioWorklet'; }; }
  if (typeof AudioWorkletNode === 'undefined') { window.AudioWorkletNode = function() { this[Symbol.toStringTag] = 'AudioWorkletNode'; }; }
  if (typeof AuthenticatorAssertionResponse === 'undefined') { window.AuthenticatorAssertionResponse = function() { this[Symbol.toStringTag] = 'AuthenticatorAssertionResponse'; }; }
  if (typeof AuthenticatorAttestationResponse === 'undefined') { window.AuthenticatorAttestationResponse = function() { this[Symbol.toStringTag] = 'AuthenticatorAttestationResponse'; }; }
  if (typeof AuthenticatorResponse === 'undefined') { window.AuthenticatorResponse = function() { this[Symbol.toStringTag] = 'AuthenticatorResponse'; }; }
  if (typeof BackgroundFetchManager === 'undefined') { window.BackgroundFetchManager = function() { this[Symbol.toStringTag] = 'BackgroundFetchManager'; }; }
  if (typeof BackgroundFetchRecord === 'undefined') { window.BackgroundFetchRecord = function() { this[Symbol.toStringTag] = 'BackgroundFetchRecord'; }; }
  if (typeof BackgroundFetchRegistration === 'undefined') { window.BackgroundFetchRegistration = function() { this[Symbol.toStringTag] = 'BackgroundFetchRegistration'; }; }
  if (typeof BarProp === 'undefined') { window.BarProp = function() { this[Symbol.toStringTag] = 'BarProp'; }; }
  if (typeof BaseAudioContext === 'undefined') { window.BaseAudioContext = function() { this[Symbol.toStringTag] = 'BaseAudioContext'; }; }
  if (typeof BatteryManager === 'undefined') { window.BatteryManager = function() { this[Symbol.toStringTag] = 'BatteryManager'; }; }
  if (typeof BeforeInstallPromptEvent === 'undefined') { window.BeforeInstallPromptEvent = function() { this[Symbol.toStringTag] = 'BeforeInstallPromptEvent'; }; }
  if (typeof BeforeUnloadEvent === 'undefined') { window.BeforeUnloadEvent = function() { this[Symbol.toStringTag] = 'BeforeUnloadEvent'; }; }
  if (typeof BiquadFilterNode === 'undefined') { window.BiquadFilterNode = function() { this[Symbol.toStringTag] = 'BiquadFilterNode'; }; }
  if (typeof BlobEvent === 'undefined') { window.BlobEvent = function() { this[Symbol.toStringTag] = 'BlobEvent'; }; }
  if (typeof Bluetooth === 'undefined') { window.Bluetooth = function() { this[Symbol.toStringTag] = 'Bluetooth'; }; }
  if (typeof BluetoothCharacteristicProperties === 'undefined') { window.BluetoothCharacteristicProperties = function() { this[Symbol.toStringTag] = 'BluetoothCharacteristicProperties'; }; }
  if (typeof BluetoothDevice === 'undefined') { window.BluetoothDevice = function() { this[Symbol.toStringTag] = 'BluetoothDevice'; }; }
  if (typeof BluetoothRemoteGATTCharacteristic === 'undefined') { window.BluetoothRemoteGATTCharacteristic = function() { this[Symbol.toStringTag] = 'BluetoothRemoteGATTCharacteristic'; }; }
  if (typeof BluetoothRemoteGATTDescriptor === 'undefined') { window.BluetoothRemoteGATTDescriptor = function() { this[Symbol.toStringTag] = 'BluetoothRemoteGATTDescriptor'; }; }
  if (typeof BluetoothRemoteGATTServer === 'undefined') { window.BluetoothRemoteGATTServer = function() { this[Symbol.toStringTag] = 'BluetoothRemoteGATTServer'; }; }
  if (typeof BluetoothRemoteGATTService === 'undefined') { window.BluetoothRemoteGATTService = function() { this[Symbol.toStringTag] = 'BluetoothRemoteGATTService'; }; }
  if (typeof BluetoothUUID === 'undefined') { window.BluetoothUUID = function() { this[Symbol.toStringTag] = 'BluetoothUUID'; }; }
  if (typeof BrowserCaptureMediaStreamTrack === 'undefined') { window.BrowserCaptureMediaStreamTrack = function() { this[Symbol.toStringTag] = 'BrowserCaptureMediaStreamTrack'; }; }
  if (typeof CDATASection === 'undefined') { window.CDATASection = function() { this[Symbol.toStringTag] = 'CDATASection'; }; }
  if (typeof CSPViolationReportBody === 'undefined') { window.CSPViolationReportBody = function() { this[Symbol.toStringTag] = 'CSPViolationReportBody'; }; }
  if (typeof CSSAnimation === 'undefined') { window.CSSAnimation = function() { this[Symbol.toStringTag] = 'CSSAnimation'; }; }
  if (typeof CSSConditionRule === 'undefined') { window.CSSConditionRule = function() { this[Symbol.toStringTag] = 'CSSConditionRule'; }; }
  if (typeof CSSContainerRule === 'undefined') { window.CSSContainerRule = function() { this[Symbol.toStringTag] = 'CSSContainerRule'; }; }
  if (typeof CSSCounterStyleRule === 'undefined') { window.CSSCounterStyleRule = function() { this[Symbol.toStringTag] = 'CSSCounterStyleRule'; }; }
  if (typeof CSSFontFaceRule === 'undefined') { window.CSSFontFaceRule = function() { this[Symbol.toStringTag] = 'CSSFontFaceRule'; }; }
  if (typeof CSSFontFeatureValuesRule === 'undefined') { window.CSSFontFeatureValuesRule = function() { this[Symbol.toStringTag] = 'CSSFontFeatureValuesRule'; }; }
  if (typeof CSSFontPaletteValuesRule === 'undefined') { window.CSSFontPaletteValuesRule = function() { this[Symbol.toStringTag] = 'CSSFontPaletteValuesRule'; }; }
  if (typeof CSSFunctionDeclarations === 'undefined') { window.CSSFunctionDeclarations = function() { this[Symbol.toStringTag] = 'CSSFunctionDeclarations'; }; }
  if (typeof CSSFunctionDescriptors === 'undefined') { window.CSSFunctionDescriptors = function() { this[Symbol.toStringTag] = 'CSSFunctionDescriptors'; }; }
  if (typeof CSSFunctionRule === 'undefined') { window.CSSFunctionRule = function() { this[Symbol.toStringTag] = 'CSSFunctionRule'; }; }
  if (typeof CSSGroupingRule === 'undefined') { window.CSSGroupingRule = function() { this[Symbol.toStringTag] = 'CSSGroupingRule'; }; }
  if (typeof CSSImageValue === 'undefined') { window.CSSImageValue = function() { this[Symbol.toStringTag] = 'CSSImageValue'; }; }
  if (typeof CSSImportRule === 'undefined') { window.CSSImportRule = function() { this[Symbol.toStringTag] = 'CSSImportRule'; }; }
  if (typeof CSSKeyframeRule === 'undefined') { window.CSSKeyframeRule = function() { this[Symbol.toStringTag] = 'CSSKeyframeRule'; }; }
  if (typeof CSSKeyframesRule === 'undefined') { window.CSSKeyframesRule = function() { this[Symbol.toStringTag] = 'CSSKeyframesRule'; }; }
  if (typeof CSSKeywordValue === 'undefined') { window.CSSKeywordValue = function() { this[Symbol.toStringTag] = 'CSSKeywordValue'; }; }
  if (typeof CSSLayerBlockRule === 'undefined') { window.CSSLayerBlockRule = function() { this[Symbol.toStringTag] = 'CSSLayerBlockRule'; }; }
  if (typeof CSSLayerStatementRule === 'undefined') { window.CSSLayerStatementRule = function() { this[Symbol.toStringTag] = 'CSSLayerStatementRule'; }; }
  if (typeof CSSMarginRule === 'undefined') { window.CSSMarginRule = function() { this[Symbol.toStringTag] = 'CSSMarginRule'; }; }
  if (typeof CSSMathClamp === 'undefined') { window.CSSMathClamp = function() { this[Symbol.toStringTag] = 'CSSMathClamp'; }; }
  if (typeof CSSMathInvert === 'undefined') { window.CSSMathInvert = function() { this[Symbol.toStringTag] = 'CSSMathInvert'; }; }
  if (typeof CSSMathMax === 'undefined') { window.CSSMathMax = function() { this[Symbol.toStringTag] = 'CSSMathMax'; }; }
  if (typeof CSSMathMin === 'undefined') { window.CSSMathMin = function() { this[Symbol.toStringTag] = 'CSSMathMin'; }; }
  if (typeof CSSMathNegate === 'undefined') { window.CSSMathNegate = function() { this[Symbol.toStringTag] = 'CSSMathNegate'; }; }
  if (typeof CSSMathProduct === 'undefined') { window.CSSMathProduct = function() { this[Symbol.toStringTag] = 'CSSMathProduct'; }; }
  if (typeof CSSMathSum === 'undefined') { window.CSSMathSum = function() { this[Symbol.toStringTag] = 'CSSMathSum'; }; }
  if (typeof CSSMathValue === 'undefined') { window.CSSMathValue = function() { this[Symbol.toStringTag] = 'CSSMathValue'; }; }
  if (typeof CSSMatrixComponent === 'undefined') { window.CSSMatrixComponent = function() { this[Symbol.toStringTag] = 'CSSMatrixComponent'; }; }
  if (typeof CSSMediaRule === 'undefined') { window.CSSMediaRule = function() { this[Symbol.toStringTag] = 'CSSMediaRule'; }; }
  if (typeof CSSNamespaceRule === 'undefined') { window.CSSNamespaceRule = function() { this[Symbol.toStringTag] = 'CSSNamespaceRule'; }; }
  if (typeof CSSNestedDeclarations === 'undefined') { window.CSSNestedDeclarations = function() { this[Symbol.toStringTag] = 'CSSNestedDeclarations'; }; }
  if (typeof CSSNumericArray === 'undefined') { window.CSSNumericArray = function() { this[Symbol.toStringTag] = 'CSSNumericArray'; }; }
  if (typeof CSSNumericValue === 'undefined') { window.CSSNumericValue = function() { this[Symbol.toStringTag] = 'CSSNumericValue'; }; }
  if (typeof CSSPageRule === 'undefined') { window.CSSPageRule = function() { this[Symbol.toStringTag] = 'CSSPageRule'; }; }
  if (typeof CSSPerspective === 'undefined') { window.CSSPerspective = function() { this[Symbol.toStringTag] = 'CSSPerspective'; }; }
  if (typeof CSSPositionTryDescriptors === 'undefined') { window.CSSPositionTryDescriptors = function() { this[Symbol.toStringTag] = 'CSSPositionTryDescriptors'; }; }
  if (typeof CSSPositionTryRule === 'undefined') { window.CSSPositionTryRule = function() { this[Symbol.toStringTag] = 'CSSPositionTryRule'; }; }
  if (typeof CSSPositionValue === 'undefined') { window.CSSPositionValue = function() { this[Symbol.toStringTag] = 'CSSPositionValue'; }; }
  if (typeof CSSPropertyRule === 'undefined') { window.CSSPropertyRule = function() { this[Symbol.toStringTag] = 'CSSPropertyRule'; }; }
  if (typeof CSSRotate === 'undefined') { window.CSSRotate = function() { this[Symbol.toStringTag] = 'CSSRotate'; }; }
  if (typeof CSSRule === 'undefined') { window.CSSRule = function() { this[Symbol.toStringTag] = 'CSSRule'; }; }
  if (typeof CSSRuleList === 'undefined') { window.CSSRuleList = function() { this[Symbol.toStringTag] = 'CSSRuleList'; }; }
  if (typeof CSSScale === 'undefined') { window.CSSScale = function() { this[Symbol.toStringTag] = 'CSSScale'; }; }
  if (typeof CSSScopeRule === 'undefined') { window.CSSScopeRule = function() { this[Symbol.toStringTag] = 'CSSScopeRule'; }; }
  if (typeof CSSSkew === 'undefined') { window.CSSSkew = function() { this[Symbol.toStringTag] = 'CSSSkew'; }; }
  if (typeof CSSSkewX === 'undefined') { window.CSSSkewX = function() { this[Symbol.toStringTag] = 'CSSSkewX'; }; }
  if (typeof CSSSkewY === 'undefined') { window.CSSSkewY = function() { this[Symbol.toStringTag] = 'CSSSkewY'; }; }
  if (typeof CSSStartingStyleRule === 'undefined') { window.CSSStartingStyleRule = function() { this[Symbol.toStringTag] = 'CSSStartingStyleRule'; }; }
  if (typeof CSSStyleDeclaration === 'undefined') { window.CSSStyleDeclaration = function() { this[Symbol.toStringTag] = 'CSSStyleDeclaration'; }; }
  if (typeof CSSStyleRule === 'undefined') { window.CSSStyleRule = function() { this[Symbol.toStringTag] = 'CSSStyleRule'; }; }
  if (typeof CSSStyleSheet === 'undefined') { window.CSSStyleSheet = function() { this[Symbol.toStringTag] = 'CSSStyleSheet'; }; }
  if (typeof CSSStyleValue === 'undefined') { window.CSSStyleValue = function() { this[Symbol.toStringTag] = 'CSSStyleValue'; }; }
  if (typeof CSSSupportsRule === 'undefined') { window.CSSSupportsRule = function() { this[Symbol.toStringTag] = 'CSSSupportsRule'; }; }
  if (typeof CSSTransformComponent === 'undefined') { window.CSSTransformComponent = function() { this[Symbol.toStringTag] = 'CSSTransformComponent'; }; }
  if (typeof CSSTransformValue === 'undefined') { window.CSSTransformValue = function() { this[Symbol.toStringTag] = 'CSSTransformValue'; }; }
  if (typeof CSSTransition === 'undefined') { window.CSSTransition = function() { this[Symbol.toStringTag] = 'CSSTransition'; }; }
  if (typeof CSSTranslate === 'undefined') { window.CSSTranslate = function() { this[Symbol.toStringTag] = 'CSSTranslate'; }; }
  if (typeof CSSUnitValue === 'undefined') { window.CSSUnitValue = function() { this[Symbol.toStringTag] = 'CSSUnitValue'; }; }
  if (typeof CSSUnparsedValue === 'undefined') { window.CSSUnparsedValue = function() { this[Symbol.toStringTag] = 'CSSUnparsedValue'; }; }
  if (typeof CSSVariableReferenceValue === 'undefined') { window.CSSVariableReferenceValue = function() { this[Symbol.toStringTag] = 'CSSVariableReferenceValue'; }; }
  if (typeof CSSViewTransitionRule === 'undefined') { window.CSSViewTransitionRule = function() { this[Symbol.toStringTag] = 'CSSViewTransitionRule'; }; }
  if (typeof Cache === 'undefined') { window.Cache = function() { this[Symbol.toStringTag] = 'Cache'; }; }
  if (typeof CacheStorage === 'undefined') { window.CacheStorage = function() { this[Symbol.toStringTag] = 'CacheStorage'; }; }
  if (typeof CanvasCaptureMediaStreamTrack === 'undefined') { window.CanvasCaptureMediaStreamTrack = function() { this[Symbol.toStringTag] = 'CanvasCaptureMediaStreamTrack'; }; }
  if (typeof CaptureController === 'undefined') { window.CaptureController = function() { this[Symbol.toStringTag] = 'CaptureController'; }; }
  if (typeof CaretPosition === 'undefined') { window.CaretPosition = function() { this[Symbol.toStringTag] = 'CaretPosition'; }; }
  if (typeof ChannelMergerNode === 'undefined') { window.ChannelMergerNode = function() { this[Symbol.toStringTag] = 'ChannelMergerNode'; }; }
  if (typeof ChannelSplitterNode === 'undefined') { window.ChannelSplitterNode = function() { this[Symbol.toStringTag] = 'ChannelSplitterNode'; }; }
  if (typeof ChapterInformation === 'undefined') { window.ChapterInformation = function() { this[Symbol.toStringTag] = 'ChapterInformation'; }; }
  if (typeof CharacterBoundsUpdateEvent === 'undefined') { window.CharacterBoundsUpdateEvent = function() { this[Symbol.toStringTag] = 'CharacterBoundsUpdateEvent'; }; }
  if (typeof CharacterData === 'undefined') { window.CharacterData = function() { this[Symbol.toStringTag] = 'CharacterData'; }; }
  if (typeof Clipboard === 'undefined') { window.Clipboard = function() { this[Symbol.toStringTag] = 'Clipboard'; }; }
  if (typeof ClipboardChangeEvent === 'undefined') { window.ClipboardChangeEvent = function() { this[Symbol.toStringTag] = 'ClipboardChangeEvent'; }; }
  if (typeof ClipboardEvent === 'undefined') { window.ClipboardEvent = function() { this[Symbol.toStringTag] = 'ClipboardEvent'; }; }
  if (typeof ClipboardItem === 'undefined') { window.ClipboardItem = function() { this[Symbol.toStringTag] = 'ClipboardItem'; }; }
  if (typeof CloseEvent === 'undefined') { window.CloseEvent = function() { this[Symbol.toStringTag] = 'CloseEvent'; }; }
  if (typeof CloseWatcher === 'undefined') { window.CloseWatcher = function() { this[Symbol.toStringTag] = 'CloseWatcher'; }; }
  if (typeof CommandEvent === 'undefined') { window.CommandEvent = function() { this[Symbol.toStringTag] = 'CommandEvent'; }; }
  if (typeof Comment === 'undefined') { window.Comment = function() { this[Symbol.toStringTag] = 'Comment'; }; }
  if (typeof CompositionEvent === 'undefined') { window.CompositionEvent = function() { this[Symbol.toStringTag] = 'CompositionEvent'; }; }
  if (typeof ConstantSourceNode === 'undefined') { window.ConstantSourceNode = function() { this[Symbol.toStringTag] = 'ConstantSourceNode'; }; }
  if (typeof ContentVisibilityAutoStateChangeEvent === 'undefined') { window.ContentVisibilityAutoStateChangeEvent = function() { this[Symbol.toStringTag] = 'ContentVisibilityAutoStateChangeEvent'; }; }
  if (typeof ConvolverNode === 'undefined') { window.ConvolverNode = function() { this[Symbol.toStringTag] = 'ConvolverNode'; }; }
  if (typeof CookieChangeEvent === 'undefined') { window.CookieChangeEvent = function() { this[Symbol.toStringTag] = 'CookieChangeEvent'; }; }
  if (typeof CookieStore === 'undefined') { window.CookieStore = function() { this[Symbol.toStringTag] = 'CookieStore'; }; }
  if (typeof CookieStoreManager === 'undefined') { window.CookieStoreManager = function() { this[Symbol.toStringTag] = 'CookieStoreManager'; }; }
  if (typeof CrashReportContext === 'undefined') { window.CrashReportContext = function() { this[Symbol.toStringTag] = 'CrashReportContext'; }; }
  if (typeof Credential === 'undefined') { window.Credential = function() { this[Symbol.toStringTag] = 'Credential'; }; }
  if (typeof CredentialsContainer === 'undefined') { window.CredentialsContainer = function() { this[Symbol.toStringTag] = 'CredentialsContainer'; }; }
  if (typeof CropTarget === 'undefined') { window.CropTarget = function() { this[Symbol.toStringTag] = 'CropTarget'; }; }
  if (typeof CryptoKey === 'undefined') { window.CryptoKey = function() { this[Symbol.toStringTag] = 'CryptoKey'; }; }
  if (typeof CustomElementRegistry === 'undefined') { window.CustomElementRegistry = function() { this[Symbol.toStringTag] = 'CustomElementRegistry'; }; }
  if (typeof CustomStateSet === 'undefined') { window.CustomStateSet = function() { this[Symbol.toStringTag] = 'CustomStateSet'; }; }
  if (typeof DOMError === 'undefined') { window.DOMError = function() { this[Symbol.toStringTag] = 'DOMError'; }; }
  if (typeof DOMImplementation === 'undefined') { window.DOMImplementation = function() { this[Symbol.toStringTag] = 'DOMImplementation'; }; }
  if (typeof DOMMatrixReadOnly === 'undefined') { window.DOMMatrixReadOnly = function() { this[Symbol.toStringTag] = 'DOMMatrixReadOnly'; }; }
  if (typeof DOMPointReadOnly === 'undefined') { window.DOMPointReadOnly = function() { this[Symbol.toStringTag] = 'DOMPointReadOnly'; }; }
  if (typeof DOMQuad === 'undefined') { window.DOMQuad = function() { this[Symbol.toStringTag] = 'DOMQuad'; }; }
  if (typeof DOMRectList === 'undefined') { window.DOMRectList = function() { this[Symbol.toStringTag] = 'DOMRectList'; }; }
  if (typeof DOMRectReadOnly === 'undefined') { window.DOMRectReadOnly = function() { this[Symbol.toStringTag] = 'DOMRectReadOnly'; }; }
  if (typeof DOMStringList === 'undefined') { window.DOMStringList = function() { this[Symbol.toStringTag] = 'DOMStringList'; }; }
  if (typeof DOMStringMap === 'undefined') { window.DOMStringMap = function() { this[Symbol.toStringTag] = 'DOMStringMap'; }; }
  if (typeof DOMTokenList === 'undefined') { window.DOMTokenList = function() { this[Symbol.toStringTag] = 'DOMTokenList'; }; }
  if (typeof DataTransfer === 'undefined') { window.DataTransfer = function() { this[Symbol.toStringTag] = 'DataTransfer'; }; }
  if (typeof DataTransferItem === 'undefined') { window.DataTransferItem = function() { this[Symbol.toStringTag] = 'DataTransferItem'; }; }
  if (typeof DataTransferItemList === 'undefined') { window.DataTransferItemList = function() { this[Symbol.toStringTag] = 'DataTransferItemList'; }; }
  if (typeof DelayNode === 'undefined') { window.DelayNode = function() { this[Symbol.toStringTag] = 'DelayNode'; }; }
  if (typeof DelegatedInkTrailPresenter === 'undefined') { window.DelegatedInkTrailPresenter = function() { this[Symbol.toStringTag] = 'DelegatedInkTrailPresenter'; }; }
  if (typeof DeviceMotionEvent === 'undefined') { window.DeviceMotionEvent = function() { this[Symbol.toStringTag] = 'DeviceMotionEvent'; }; }
  if (typeof DeviceMotionEventAcceleration === 'undefined') { window.DeviceMotionEventAcceleration = function() { this[Symbol.toStringTag] = 'DeviceMotionEventAcceleration'; }; }
  if (typeof DeviceMotionEventRotationRate === 'undefined') { window.DeviceMotionEventRotationRate = function() { this[Symbol.toStringTag] = 'DeviceMotionEventRotationRate'; }; }
  if (typeof DeviceOrientationEvent === 'undefined') { window.DeviceOrientationEvent = function() { this[Symbol.toStringTag] = 'DeviceOrientationEvent'; }; }
  if (typeof DevicePosture === 'undefined') { window.DevicePosture = function() { this[Symbol.toStringTag] = 'DevicePosture'; }; }
  if (typeof DigitalCredential === 'undefined') { window.DigitalCredential = function() { this[Symbol.toStringTag] = 'DigitalCredential'; }; }
  if (typeof DisposableStack === 'undefined') { window.DisposableStack = function() { this[Symbol.toStringTag] = 'DisposableStack'; }; }
  if (typeof DocumentFragment === 'undefined') { window.DocumentFragment = function() { this[Symbol.toStringTag] = 'DocumentFragment'; }; }
  if (typeof DocumentPictureInPicture === 'undefined') { window.DocumentPictureInPicture = function() { this[Symbol.toStringTag] = 'DocumentPictureInPicture'; }; }
  if (typeof DocumentPictureInPictureEvent === 'undefined') { window.DocumentPictureInPictureEvent = function() { this[Symbol.toStringTag] = 'DocumentPictureInPictureEvent'; }; }
  if (typeof DocumentTimeline === 'undefined') { window.DocumentTimeline = function() { this[Symbol.toStringTag] = 'DocumentTimeline'; }; }
  if (typeof DocumentType === 'undefined') { window.DocumentType = function() { this[Symbol.toStringTag] = 'DocumentType'; }; }
  if (typeof DragEvent === 'undefined') { window.DragEvent = function() { this[Symbol.toStringTag] = 'DragEvent'; }; }
  if (typeof DynamicsCompressorNode === 'undefined') { window.DynamicsCompressorNode = function() { this[Symbol.toStringTag] = 'DynamicsCompressorNode'; }; }
  if (typeof EditContext === 'undefined') { window.EditContext = function() { this[Symbol.toStringTag] = 'EditContext'; }; }
  if (typeof ElementInternals === 'undefined') { window.ElementInternals = function() { this[Symbol.toStringTag] = 'ElementInternals'; }; }
  if (typeof EncodedAudioChunk === 'undefined') { window.EncodedAudioChunk = function() { this[Symbol.toStringTag] = 'EncodedAudioChunk'; }; }
  if (typeof EncodedVideoChunk === 'undefined') { window.EncodedVideoChunk = function() { this[Symbol.toStringTag] = 'EncodedVideoChunk'; }; }
  if (typeof EventCounts === 'undefined') { window.EventCounts = function() { this[Symbol.toStringTag] = 'EventCounts'; }; }
  if (typeof External === 'undefined') { window.External = function() { this[Symbol.toStringTag] = 'External'; }; }
  if (typeof EyeDropper === 'undefined') { window.EyeDropper = function() { this[Symbol.toStringTag] = 'EyeDropper'; }; }
  if (typeof FeaturePolicy === 'undefined') { window.FeaturePolicy = function() { this[Symbol.toStringTag] = 'FeaturePolicy'; }; }
  if (typeof FederatedCredential === 'undefined') { window.FederatedCredential = function() { this[Symbol.toStringTag] = 'FederatedCredential'; }; }
  if (typeof Fence === 'undefined') { window.Fence = function() { this[Symbol.toStringTag] = 'Fence'; }; }
  if (typeof FencedFrameConfig === 'undefined') { window.FencedFrameConfig = function() { this[Symbol.toStringTag] = 'FencedFrameConfig'; }; }
  if (typeof FetchLaterResult === 'undefined') { window.FetchLaterResult = function() { this[Symbol.toStringTag] = 'FetchLaterResult'; }; }
  if (typeof FileSystemDirectoryHandle === 'undefined') { window.FileSystemDirectoryHandle = function() { this[Symbol.toStringTag] = 'FileSystemDirectoryHandle'; }; }
  if (typeof FileSystemFileHandle === 'undefined') { window.FileSystemFileHandle = function() { this[Symbol.toStringTag] = 'FileSystemFileHandle'; }; }
  if (typeof FileSystemHandle === 'undefined') { window.FileSystemHandle = function() { this[Symbol.toStringTag] = 'FileSystemHandle'; }; }
  if (typeof FileSystemObserver === 'undefined') { window.FileSystemObserver = function() { this[Symbol.toStringTag] = 'FileSystemObserver'; }; }
  if (typeof FileSystemWritableFileStream === 'undefined') { window.FileSystemWritableFileStream = function() { this[Symbol.toStringTag] = 'FileSystemWritableFileStream'; }; }
  if (typeof Float16Array === 'undefined') { window.Float16Array = function() { this[Symbol.toStringTag] = 'Float16Array'; }; }
  if (typeof FocusEvent === 'undefined') { window.FocusEvent = function() { this[Symbol.toStringTag] = 'FocusEvent'; }; }
  if (typeof FontData === 'undefined') { window.FontData = function() { this[Symbol.toStringTag] = 'FontData'; }; }
  if (typeof FontFaceSetLoadEvent === 'undefined') { window.FontFaceSetLoadEvent = function() { this[Symbol.toStringTag] = 'FontFaceSetLoadEvent'; }; }
  if (typeof FormDataEvent === 'undefined') { window.FormDataEvent = function() { this[Symbol.toStringTag] = 'FormDataEvent'; }; }
  if (typeof FragmentDirective === 'undefined') { window.FragmentDirective = function() { this[Symbol.toStringTag] = 'FragmentDirective'; }; }
  if (typeof GPU === 'undefined') { window.GPU = function() { this[Symbol.toStringTag] = 'GPU'; }; }
  if (typeof GPUAdapter === 'undefined') { window.GPUAdapter = function() { this[Symbol.toStringTag] = 'GPUAdapter'; }; }
  if (typeof GPUAdapterInfo === 'undefined') { window.GPUAdapterInfo = function() { this[Symbol.toStringTag] = 'GPUAdapterInfo'; }; }
  if (typeof GPUBindGroup === 'undefined') { window.GPUBindGroup = function() { this[Symbol.toStringTag] = 'GPUBindGroup'; }; }
  if (typeof GPUBindGroupLayout === 'undefined') { window.GPUBindGroupLayout = function() { this[Symbol.toStringTag] = 'GPUBindGroupLayout'; }; }
  if (typeof GPUBuffer === 'undefined') { window.GPUBuffer = function() { this[Symbol.toStringTag] = 'GPUBuffer'; }; }
  if (typeof GPUCanvasContext === 'undefined') { window.GPUCanvasContext = function() { this[Symbol.toStringTag] = 'GPUCanvasContext'; }; }
  if (typeof GPUCommandBuffer === 'undefined') { window.GPUCommandBuffer = function() { this[Symbol.toStringTag] = 'GPUCommandBuffer'; }; }
  if (typeof GPUCommandEncoder === 'undefined') { window.GPUCommandEncoder = function() { this[Symbol.toStringTag] = 'GPUCommandEncoder'; }; }
  if (typeof GPUCompilationInfo === 'undefined') { window.GPUCompilationInfo = function() { this[Symbol.toStringTag] = 'GPUCompilationInfo'; }; }
  if (typeof GPUCompilationMessage === 'undefined') { window.GPUCompilationMessage = function() { this[Symbol.toStringTag] = 'GPUCompilationMessage'; }; }
  if (typeof GPUComputePassEncoder === 'undefined') { window.GPUComputePassEncoder = function() { this[Symbol.toStringTag] = 'GPUComputePassEncoder'; }; }
  if (typeof GPUComputePipeline === 'undefined') { window.GPUComputePipeline = function() { this[Symbol.toStringTag] = 'GPUComputePipeline'; }; }
  if (typeof GPUDevice === 'undefined') { window.GPUDevice = function() { this[Symbol.toStringTag] = 'GPUDevice'; }; }
  if (typeof GPUDeviceLostInfo === 'undefined') { window.GPUDeviceLostInfo = function() { this[Symbol.toStringTag] = 'GPUDeviceLostInfo'; }; }
  if (typeof GPUError === 'undefined') { window.GPUError = function() { this[Symbol.toStringTag] = 'GPUError'; }; }
  if (typeof GPUExternalTexture === 'undefined') { window.GPUExternalTexture = function() { this[Symbol.toStringTag] = 'GPUExternalTexture'; }; }
  if (typeof GPUInternalError === 'undefined') { window.GPUInternalError = function() { this[Symbol.toStringTag] = 'GPUInternalError'; }; }
  if (typeof GPUOutOfMemoryError === 'undefined') { window.GPUOutOfMemoryError = function() { this[Symbol.toStringTag] = 'GPUOutOfMemoryError'; }; }
  if (typeof GPUPipelineError === 'undefined') { window.GPUPipelineError = function() { this[Symbol.toStringTag] = 'GPUPipelineError'; }; }
  if (typeof GPUPipelineLayout === 'undefined') { window.GPUPipelineLayout = function() { this[Symbol.toStringTag] = 'GPUPipelineLayout'; }; }
  if (typeof GPUQuerySet === 'undefined') { window.GPUQuerySet = function() { this[Symbol.toStringTag] = 'GPUQuerySet'; }; }
  if (typeof GPUQueue === 'undefined') { window.GPUQueue = function() { this[Symbol.toStringTag] = 'GPUQueue'; }; }
  if (typeof GPURenderBundle === 'undefined') { window.GPURenderBundle = function() { this[Symbol.toStringTag] = 'GPURenderBundle'; }; }
  if (typeof GPURenderBundleEncoder === 'undefined') { window.GPURenderBundleEncoder = function() { this[Symbol.toStringTag] = 'GPURenderBundleEncoder'; }; }
  if (typeof GPURenderPassEncoder === 'undefined') { window.GPURenderPassEncoder = function() { this[Symbol.toStringTag] = 'GPURenderPassEncoder'; }; }
  if (typeof GPURenderPipeline === 'undefined') { window.GPURenderPipeline = function() { this[Symbol.toStringTag] = 'GPURenderPipeline'; }; }
  if (typeof GPUSampler === 'undefined') { window.GPUSampler = function() { this[Symbol.toStringTag] = 'GPUSampler'; }; }
  if (typeof GPUShaderModule === 'undefined') { window.GPUShaderModule = function() { this[Symbol.toStringTag] = 'GPUShaderModule'; }; }
  if (typeof GPUSupportedFeatures === 'undefined') { window.GPUSupportedFeatures = function() { this[Symbol.toStringTag] = 'GPUSupportedFeatures'; }; }
  if (typeof GPUSupportedLimits === 'undefined') { window.GPUSupportedLimits = function() { this[Symbol.toStringTag] = 'GPUSupportedLimits'; }; }
  if (typeof GPUTexture === 'undefined') { window.GPUTexture = function() { this[Symbol.toStringTag] = 'GPUTexture'; }; }
  if (typeof GPUTextureView === 'undefined') { window.GPUTextureView = function() { this[Symbol.toStringTag] = 'GPUTextureView'; }; }
  if (typeof GPUUncapturedErrorEvent === 'undefined') { window.GPUUncapturedErrorEvent = function() { this[Symbol.toStringTag] = 'GPUUncapturedErrorEvent'; }; }
  if (typeof GPUValidationError === 'undefined') { window.GPUValidationError = function() { this[Symbol.toStringTag] = 'GPUValidationError'; }; }
  if (typeof GainNode === 'undefined') { window.GainNode = function() { this[Symbol.toStringTag] = 'GainNode'; }; }
  if (typeof Gamepad === 'undefined') { window.Gamepad = function() { this[Symbol.toStringTag] = 'Gamepad'; }; }
  if (typeof GamepadButton === 'undefined') { window.GamepadButton = function() { this[Symbol.toStringTag] = 'GamepadButton'; }; }
  if (typeof GamepadEvent === 'undefined') { window.GamepadEvent = function() { this[Symbol.toStringTag] = 'GamepadEvent'; }; }
  if (typeof GamepadHapticActuator === 'undefined') { window.GamepadHapticActuator = function() { this[Symbol.toStringTag] = 'GamepadHapticActuator'; }; }
  if (typeof Geolocation === 'undefined') { window.Geolocation = function() { this[Symbol.toStringTag] = 'Geolocation'; }; }
  if (typeof GeolocationCoordinates === 'undefined') { window.GeolocationCoordinates = function() { this[Symbol.toStringTag] = 'GeolocationCoordinates'; }; }
  if (typeof GeolocationPosition === 'undefined') { window.GeolocationPosition = function() { this[Symbol.toStringTag] = 'GeolocationPosition'; }; }
  if (typeof GeolocationPositionError === 'undefined') { window.GeolocationPositionError = function() { this[Symbol.toStringTag] = 'GeolocationPositionError'; }; }
  if (typeof GravitySensor === 'undefined') { window.GravitySensor = function() { this[Symbol.toStringTag] = 'GravitySensor'; }; }
  if (typeof Gyroscope === 'undefined') { window.Gyroscope = function() { this[Symbol.toStringTag] = 'Gyroscope'; }; }
  if (typeof HID === 'undefined') { window.HID = function() { this[Symbol.toStringTag] = 'HID'; }; }
  if (typeof HIDConnectionEvent === 'undefined') { window.HIDConnectionEvent = function() { this[Symbol.toStringTag] = 'HIDConnectionEvent'; }; }
  if (typeof HIDDevice === 'undefined') { window.HIDDevice = function() { this[Symbol.toStringTag] = 'HIDDevice'; }; }
  if (typeof HIDInputReportEvent === 'undefined') { window.HIDInputReportEvent = function() { this[Symbol.toStringTag] = 'HIDInputReportEvent'; }; }
  if (typeof HTMLAllCollection === 'undefined') { window.HTMLAllCollection = function() { this[Symbol.toStringTag] = 'HTMLAllCollection'; }; }
  if (typeof HTMLAnchorElement === 'undefined') { window.HTMLAnchorElement = function() { this[Symbol.toStringTag] = 'HTMLAnchorElement'; }; }
  if (typeof HTMLAreaElement === 'undefined') { window.HTMLAreaElement = function() { this[Symbol.toStringTag] = 'HTMLAreaElement'; }; }
  if (typeof HTMLAudioElement === 'undefined') { window.HTMLAudioElement = function() { this[Symbol.toStringTag] = 'HTMLAudioElement'; }; }
  if (typeof HTMLBRElement === 'undefined') { window.HTMLBRElement = function() { this[Symbol.toStringTag] = 'HTMLBRElement'; }; }
  if (typeof HTMLBaseElement === 'undefined') { window.HTMLBaseElement = function() { this[Symbol.toStringTag] = 'HTMLBaseElement'; }; }
  if (typeof HTMLBodyElement === 'undefined') { window.HTMLBodyElement = function() { this[Symbol.toStringTag] = 'HTMLBodyElement'; }; }
  if (typeof HTMLButtonElement === 'undefined') { window.HTMLButtonElement = function() { this[Symbol.toStringTag] = 'HTMLButtonElement'; }; }
  if (typeof HTMLCanvasElement === 'undefined') { window.HTMLCanvasElement = function() { this[Symbol.toStringTag] = 'HTMLCanvasElement'; }; }
  if (typeof HTMLDListElement === 'undefined') { window.HTMLDListElement = function() { this[Symbol.toStringTag] = 'HTMLDListElement'; }; }
  if (typeof HTMLDataElement === 'undefined') { window.HTMLDataElement = function() { this[Symbol.toStringTag] = 'HTMLDataElement'; }; }
  if (typeof HTMLDataListElement === 'undefined') { window.HTMLDataListElement = function() { this[Symbol.toStringTag] = 'HTMLDataListElement'; }; }
  if (typeof HTMLDetailsElement === 'undefined') { window.HTMLDetailsElement = function() { this[Symbol.toStringTag] = 'HTMLDetailsElement'; }; }
  if (typeof HTMLDialogElement === 'undefined') { window.HTMLDialogElement = function() { this[Symbol.toStringTag] = 'HTMLDialogElement'; }; }
  if (typeof HTMLDirectoryElement === 'undefined') { window.HTMLDirectoryElement = function() { this[Symbol.toStringTag] = 'HTMLDirectoryElement'; }; }
  if (typeof HTMLDivElement === 'undefined') { window.HTMLDivElement = function() { this[Symbol.toStringTag] = 'HTMLDivElement'; }; }
  if (typeof HTMLDocument === 'undefined') { window.HTMLDocument = function() { this[Symbol.toStringTag] = 'HTMLDocument'; }; }
  if (typeof HTMLElement === 'undefined') { window.HTMLElement = function() { this[Symbol.toStringTag] = 'HTMLElement'; }; }
  if (typeof HTMLEmbedElement === 'undefined') { window.HTMLEmbedElement = function() { this[Symbol.toStringTag] = 'HTMLEmbedElement'; }; }
  if (typeof HTMLFencedFrameElement === 'undefined') { window.HTMLFencedFrameElement = function() { this[Symbol.toStringTag] = 'HTMLFencedFrameElement'; }; }
  if (typeof HTMLFieldSetElement === 'undefined') { window.HTMLFieldSetElement = function() { this[Symbol.toStringTag] = 'HTMLFieldSetElement'; }; }
  if (typeof HTMLFontElement === 'undefined') { window.HTMLFontElement = function() { this[Symbol.toStringTag] = 'HTMLFontElement'; }; }
  if (typeof HTMLFormControlsCollection === 'undefined') { window.HTMLFormControlsCollection = function() { this[Symbol.toStringTag] = 'HTMLFormControlsCollection'; }; }
  if (typeof HTMLFormElement === 'undefined') { window.HTMLFormElement = function() { this[Symbol.toStringTag] = 'HTMLFormElement'; }; }
  if (typeof HTMLFrameElement === 'undefined') { window.HTMLFrameElement = function() { this[Symbol.toStringTag] = 'HTMLFrameElement'; }; }
  if (typeof HTMLFrameSetElement === 'undefined') { window.HTMLFrameSetElement = function() { this[Symbol.toStringTag] = 'HTMLFrameSetElement'; }; }
  if (typeof HTMLGeolocationElement === 'undefined') { window.HTMLGeolocationElement = function() { this[Symbol.toStringTag] = 'HTMLGeolocationElement'; }; }
  if (typeof HTMLHRElement === 'undefined') { window.HTMLHRElement = function() { this[Symbol.toStringTag] = 'HTMLHRElement'; }; }
  if (typeof HTMLHeadElement === 'undefined') { window.HTMLHeadElement = function() { this[Symbol.toStringTag] = 'HTMLHeadElement'; }; }
  if (typeof HTMLHeadingElement === 'undefined') { window.HTMLHeadingElement = function() { this[Symbol.toStringTag] = 'HTMLHeadingElement'; }; }
  if (typeof HTMLHtmlElement === 'undefined') { window.HTMLHtmlElement = function() { this[Symbol.toStringTag] = 'HTMLHtmlElement'; }; }
  if (typeof HTMLIFrameElement === 'undefined') { window.HTMLIFrameElement = function() { this[Symbol.toStringTag] = 'HTMLIFrameElement'; }; }
  if (typeof HTMLImageElement === 'undefined') { window.HTMLImageElement = function() { this[Symbol.toStringTag] = 'HTMLImageElement'; }; }
  if (typeof HTMLInputElement === 'undefined') { window.HTMLInputElement = function() { this[Symbol.toStringTag] = 'HTMLInputElement'; }; }
  if (typeof HTMLLIElement === 'undefined') { window.HTMLLIElement = function() { this[Symbol.toStringTag] = 'HTMLLIElement'; }; }
  if (typeof HTMLLabelElement === 'undefined') { window.HTMLLabelElement = function() { this[Symbol.toStringTag] = 'HTMLLabelElement'; }; }
  if (typeof HTMLLegendElement === 'undefined') { window.HTMLLegendElement = function() { this[Symbol.toStringTag] = 'HTMLLegendElement'; }; }
  if (typeof HTMLLinkElement === 'undefined') { window.HTMLLinkElement = function() { this[Symbol.toStringTag] = 'HTMLLinkElement'; }; }
  if (typeof HTMLMapElement === 'undefined') { window.HTMLMapElement = function() { this[Symbol.toStringTag] = 'HTMLMapElement'; }; }
  if (typeof HTMLMarqueeElement === 'undefined') { window.HTMLMarqueeElement = function() { this[Symbol.toStringTag] = 'HTMLMarqueeElement'; }; }
  if (typeof HTMLMediaElement === 'undefined') { window.HTMLMediaElement = function() { this[Symbol.toStringTag] = 'HTMLMediaElement'; }; }
  if (typeof HTMLMenuElement === 'undefined') { window.HTMLMenuElement = function() { this[Symbol.toStringTag] = 'HTMLMenuElement'; }; }
  if (typeof HTMLMetaElement === 'undefined') { window.HTMLMetaElement = function() { this[Symbol.toStringTag] = 'HTMLMetaElement'; }; }
  if (typeof HTMLMeterElement === 'undefined') { window.HTMLMeterElement = function() { this[Symbol.toStringTag] = 'HTMLMeterElement'; }; }
  if (typeof HTMLModElement === 'undefined') { window.HTMLModElement = function() { this[Symbol.toStringTag] = 'HTMLModElement'; }; }
  if (typeof HTMLOListElement === 'undefined') { window.HTMLOListElement = function() { this[Symbol.toStringTag] = 'HTMLOListElement'; }; }
  if (typeof HTMLObjectElement === 'undefined') { window.HTMLObjectElement = function() { this[Symbol.toStringTag] = 'HTMLObjectElement'; }; }
  if (typeof HTMLOptGroupElement === 'undefined') { window.HTMLOptGroupElement = function() { this[Symbol.toStringTag] = 'HTMLOptGroupElement'; }; }
  if (typeof HTMLOptionElement === 'undefined') { window.HTMLOptionElement = function() { this[Symbol.toStringTag] = 'HTMLOptionElement'; }; }
  if (typeof HTMLOptionsCollection === 'undefined') { window.HTMLOptionsCollection = function() { this[Symbol.toStringTag] = 'HTMLOptionsCollection'; }; }
  if (typeof HTMLOutputElement === 'undefined') { window.HTMLOutputElement = function() { this[Symbol.toStringTag] = 'HTMLOutputElement'; }; }
  if (typeof HTMLParagraphElement === 'undefined') { window.HTMLParagraphElement = function() { this[Symbol.toStringTag] = 'HTMLParagraphElement'; }; }
  if (typeof HTMLParamElement === 'undefined') { window.HTMLParamElement = function() { this[Symbol.toStringTag] = 'HTMLParamElement'; }; }
  if (typeof HTMLPictureElement === 'undefined') { window.HTMLPictureElement = function() { this[Symbol.toStringTag] = 'HTMLPictureElement'; }; }
  if (typeof HTMLPreElement === 'undefined') { window.HTMLPreElement = function() { this[Symbol.toStringTag] = 'HTMLPreElement'; }; }
  if (typeof HTMLProgressElement === 'undefined') { window.HTMLProgressElement = function() { this[Symbol.toStringTag] = 'HTMLProgressElement'; }; }
  if (typeof HTMLQuoteElement === 'undefined') { window.HTMLQuoteElement = function() { this[Symbol.toStringTag] = 'HTMLQuoteElement'; }; }
  if (typeof HTMLScriptElement === 'undefined') { window.HTMLScriptElement = function() { this[Symbol.toStringTag] = 'HTMLScriptElement'; }; }
  if (typeof HTMLSelectElement === 'undefined') { window.HTMLSelectElement = function() { this[Symbol.toStringTag] = 'HTMLSelectElement'; }; }
  if (typeof HTMLSelectedContentElement === 'undefined') { window.HTMLSelectedContentElement = function() { this[Symbol.toStringTag] = 'HTMLSelectedContentElement'; }; }
  if (typeof HTMLSlotElement === 'undefined') { window.HTMLSlotElement = function() { this[Symbol.toStringTag] = 'HTMLSlotElement'; }; }
  if (typeof HTMLSourceElement === 'undefined') { window.HTMLSourceElement = function() { this[Symbol.toStringTag] = 'HTMLSourceElement'; }; }
  if (typeof HTMLSpanElement === 'undefined') { window.HTMLSpanElement = function() { this[Symbol.toStringTag] = 'HTMLSpanElement'; }; }
  if (typeof HTMLStyleElement === 'undefined') { window.HTMLStyleElement = function() { this[Symbol.toStringTag] = 'HTMLStyleElement'; }; }
  if (typeof HTMLTableCaptionElement === 'undefined') { window.HTMLTableCaptionElement = function() { this[Symbol.toStringTag] = 'HTMLTableCaptionElement'; }; }
  if (typeof HTMLTableCellElement === 'undefined') { window.HTMLTableCellElement = function() { this[Symbol.toStringTag] = 'HTMLTableCellElement'; }; }
  if (typeof HTMLTableColElement === 'undefined') { window.HTMLTableColElement = function() { this[Symbol.toStringTag] = 'HTMLTableColElement'; }; }
  if (typeof HTMLTableElement === 'undefined') { window.HTMLTableElement = function() { this[Symbol.toStringTag] = 'HTMLTableElement'; }; }
  if (typeof HTMLTableRowElement === 'undefined') { window.HTMLTableRowElement = function() { this[Symbol.toStringTag] = 'HTMLTableRowElement'; }; }
  if (typeof HTMLTableSectionElement === 'undefined') { window.HTMLTableSectionElement = function() { this[Symbol.toStringTag] = 'HTMLTableSectionElement'; }; }
  if (typeof HTMLTemplateElement === 'undefined') { window.HTMLTemplateElement = function() { this[Symbol.toStringTag] = 'HTMLTemplateElement'; }; }
  if (typeof HTMLTextAreaElement === 'undefined') { window.HTMLTextAreaElement = function() { this[Symbol.toStringTag] = 'HTMLTextAreaElement'; }; }
  if (typeof HTMLTimeElement === 'undefined') { window.HTMLTimeElement = function() { this[Symbol.toStringTag] = 'HTMLTimeElement'; }; }
  if (typeof HTMLTitleElement === 'undefined') { window.HTMLTitleElement = function() { this[Symbol.toStringTag] = 'HTMLTitleElement'; }; }
  if (typeof HTMLTrackElement === 'undefined') { window.HTMLTrackElement = function() { this[Symbol.toStringTag] = 'HTMLTrackElement'; }; }
  if (typeof HTMLUListElement === 'undefined') { window.HTMLUListElement = function() { this[Symbol.toStringTag] = 'HTMLUListElement'; }; }
  if (typeof HTMLUnknownElement === 'undefined') { window.HTMLUnknownElement = function() { this[Symbol.toStringTag] = 'HTMLUnknownElement'; }; }
  if (typeof HTMLVideoElement === 'undefined') { window.HTMLVideoElement = function() { this[Symbol.toStringTag] = 'HTMLVideoElement'; }; }
  if (typeof HashChangeEvent === 'undefined') { window.HashChangeEvent = function() { this[Symbol.toStringTag] = 'HashChangeEvent'; }; }
  if (typeof Highlight === 'undefined') { window.Highlight = function() { this[Symbol.toStringTag] = 'Highlight'; }; }
  if (typeof HighlightRegistry === 'undefined') { window.HighlightRegistry = function() { this[Symbol.toStringTag] = 'HighlightRegistry'; }; }
  if (typeof IDBCursor === 'undefined') { window.IDBCursor = function() { this[Symbol.toStringTag] = 'IDBCursor'; }; }
  if (typeof IDBCursorWithValue === 'undefined') { window.IDBCursorWithValue = function() { this[Symbol.toStringTag] = 'IDBCursorWithValue'; }; }
  if (typeof IDBDatabase === 'undefined') { window.IDBDatabase = function() { this[Symbol.toStringTag] = 'IDBDatabase'; }; }
  if (typeof IDBFactory === 'undefined') { window.IDBFactory = function() { this[Symbol.toStringTag] = 'IDBFactory'; }; }
  if (typeof IDBIndex === 'undefined') { window.IDBIndex = function() { this[Symbol.toStringTag] = 'IDBIndex'; }; }
  if (typeof IDBKeyRange === 'undefined') { window.IDBKeyRange = function() { this[Symbol.toStringTag] = 'IDBKeyRange'; }; }
  if (typeof IDBObjectStore === 'undefined') { window.IDBObjectStore = function() { this[Symbol.toStringTag] = 'IDBObjectStore'; }; }
  if (typeof IDBOpenDBRequest === 'undefined') { window.IDBOpenDBRequest = function() { this[Symbol.toStringTag] = 'IDBOpenDBRequest'; }; }
  if (typeof IDBRecord === 'undefined') { window.IDBRecord = function() { this[Symbol.toStringTag] = 'IDBRecord'; }; }
  if (typeof IDBRequest === 'undefined') { window.IDBRequest = function() { this[Symbol.toStringTag] = 'IDBRequest'; }; }
  if (typeof IDBTransaction === 'undefined') { window.IDBTransaction = function() { this[Symbol.toStringTag] = 'IDBTransaction'; }; }
  if (typeof IDBVersionChangeEvent === 'undefined') { window.IDBVersionChangeEvent = function() { this[Symbol.toStringTag] = 'IDBVersionChangeEvent'; }; }
  if (typeof IIRFilterNode === 'undefined') { window.IIRFilterNode = function() { this[Symbol.toStringTag] = 'IIRFilterNode'; }; }
  if (typeof IdentityCredential === 'undefined') { window.IdentityCredential = function() { this[Symbol.toStringTag] = 'IdentityCredential'; }; }
  if (typeof IdentityCredentialError === 'undefined') { window.IdentityCredentialError = function() { this[Symbol.toStringTag] = 'IdentityCredentialError'; }; }
  if (typeof IdentityProvider === 'undefined') { window.IdentityProvider = function() { this[Symbol.toStringTag] = 'IdentityProvider'; }; }
  if (typeof IdleDeadline === 'undefined') { window.IdleDeadline = function() { this[Symbol.toStringTag] = 'IdleDeadline'; }; }
  if (typeof IdleDetector === 'undefined') { window.IdleDetector = function() { this[Symbol.toStringTag] = 'IdleDetector'; }; }
  if (typeof ImageBitmap === 'undefined') { window.ImageBitmap = function() { this[Symbol.toStringTag] = 'ImageBitmap'; }; }
  if (typeof ImageBitmapRenderingContext === 'undefined') { window.ImageBitmapRenderingContext = function() { this[Symbol.toStringTag] = 'ImageBitmapRenderingContext'; }; }
  if (typeof ImageCapture === 'undefined') { window.ImageCapture = function() { this[Symbol.toStringTag] = 'ImageCapture'; }; }
  if (typeof ImageDecoder === 'undefined') { window.ImageDecoder = function() { this[Symbol.toStringTag] = 'ImageDecoder'; }; }
  if (typeof ImageTrack === 'undefined') { window.ImageTrack = function() { this[Symbol.toStringTag] = 'ImageTrack'; }; }
  if (typeof ImageTrackList === 'undefined') { window.ImageTrackList = function() { this[Symbol.toStringTag] = 'ImageTrackList'; }; }
  if (typeof Ink === 'undefined') { window.Ink = function() { this[Symbol.toStringTag] = 'Ink'; }; }
  if (typeof InputDeviceCapabilities === 'undefined') { window.InputDeviceCapabilities = function() { this[Symbol.toStringTag] = 'InputDeviceCapabilities'; }; }
  if (typeof InputDeviceInfo === 'undefined') { window.InputDeviceInfo = function() { this[Symbol.toStringTag] = 'InputDeviceInfo'; }; }
  if (typeof InputEvent === 'undefined') { window.InputEvent = function() { this[Symbol.toStringTag] = 'InputEvent'; }; }
  if (typeof IntegrityViolationReportBody === 'undefined') { window.IntegrityViolationReportBody = function() { this[Symbol.toStringTag] = 'IntegrityViolationReportBody'; }; }
  if (typeof InterestEvent === 'undefined') { window.InterestEvent = function() { this[Symbol.toStringTag] = 'InterestEvent'; }; }
  if (typeof IntersectionObserverEntry === 'undefined') { window.IntersectionObserverEntry = function() { this[Symbol.toStringTag] = 'IntersectionObserverEntry'; }; }
  if (typeof Keyboard === 'undefined') { window.Keyboard = function() { this[Symbol.toStringTag] = 'Keyboard'; }; }
  if (typeof KeyboardEvent === 'undefined') { window.KeyboardEvent = function() { this[Symbol.toStringTag] = 'KeyboardEvent'; }; }
  if (typeof KeyboardLayoutMap === 'undefined') { window.KeyboardLayoutMap = function() { this[Symbol.toStringTag] = 'KeyboardLayoutMap'; }; }
  if (typeof KeyframeEffect === 'undefined') { window.KeyframeEffect = function() { this[Symbol.toStringTag] = 'KeyframeEffect'; }; }
  if (typeof LanguageDetector === 'undefined') { window.LanguageDetector = function() { this[Symbol.toStringTag] = 'LanguageDetector'; }; }
  if (typeof LargestContentfulPaint === 'undefined') { window.LargestContentfulPaint = function() { this[Symbol.toStringTag] = 'LargestContentfulPaint'; }; }
  if (typeof LaunchParams === 'undefined') { window.LaunchParams = function() { this[Symbol.toStringTag] = 'LaunchParams'; }; }
  if (typeof LaunchQueue === 'undefined') { window.LaunchQueue = function() { this[Symbol.toStringTag] = 'LaunchQueue'; }; }
  if (typeof LayoutShift === 'undefined') { window.LayoutShift = function() { this[Symbol.toStringTag] = 'LayoutShift'; }; }
  if (typeof LayoutShiftAttribution === 'undefined') { window.LayoutShiftAttribution = function() { this[Symbol.toStringTag] = 'LayoutShiftAttribution'; }; }
  if (typeof LinearAccelerationSensor === 'undefined') { window.LinearAccelerationSensor = function() { this[Symbol.toStringTag] = 'LinearAccelerationSensor'; }; }
  if (typeof Lock === 'undefined') { window.Lock = function() { this[Symbol.toStringTag] = 'Lock'; }; }
  if (typeof LockManager === 'undefined') { window.LockManager = function() { this[Symbol.toStringTag] = 'LockManager'; }; }
  if (typeof MIDIAccess === 'undefined') { window.MIDIAccess = function() { this[Symbol.toStringTag] = 'MIDIAccess'; }; }
  if (typeof MIDIConnectionEvent === 'undefined') { window.MIDIConnectionEvent = function() { this[Symbol.toStringTag] = 'MIDIConnectionEvent'; }; }
  if (typeof MIDIInput === 'undefined') { window.MIDIInput = function() { this[Symbol.toStringTag] = 'MIDIInput'; }; }
  if (typeof MIDIInputMap === 'undefined') { window.MIDIInputMap = function() { this[Symbol.toStringTag] = 'MIDIInputMap'; }; }
  if (typeof MIDIMessageEvent === 'undefined') { window.MIDIMessageEvent = function() { this[Symbol.toStringTag] = 'MIDIMessageEvent'; }; }
  if (typeof MIDIOutput === 'undefined') { window.MIDIOutput = function() { this[Symbol.toStringTag] = 'MIDIOutput'; }; }
  if (typeof MIDIOutputMap === 'undefined') { window.MIDIOutputMap = function() { this[Symbol.toStringTag] = 'MIDIOutputMap'; }; }
  if (typeof MIDIPort === 'undefined') { window.MIDIPort = function() { this[Symbol.toStringTag] = 'MIDIPort'; }; }
  if (typeof MathMLElement === 'undefined') { window.MathMLElement = function() { this[Symbol.toStringTag] = 'MathMLElement'; }; }
  if (typeof MediaCapabilities === 'undefined') { window.MediaCapabilities = function() { this[Symbol.toStringTag] = 'MediaCapabilities'; }; }
  if (typeof MediaDeviceInfo === 'undefined') { window.MediaDeviceInfo = function() { this[Symbol.toStringTag] = 'MediaDeviceInfo'; }; }
  if (typeof MediaDevices === 'undefined') { window.MediaDevices = function() { this[Symbol.toStringTag] = 'MediaDevices'; }; }
  if (typeof MediaElementAudioSourceNode === 'undefined') { window.MediaElementAudioSourceNode = function() { this[Symbol.toStringTag] = 'MediaElementAudioSourceNode'; }; }
  if (typeof MediaEncryptedEvent === 'undefined') { window.MediaEncryptedEvent = function() { this[Symbol.toStringTag] = 'MediaEncryptedEvent'; }; }
  if (typeof MediaError === 'undefined') { window.MediaError = function() { this[Symbol.toStringTag] = 'MediaError'; }; }
  if (typeof MediaKeyMessageEvent === 'undefined') { window.MediaKeyMessageEvent = function() { this[Symbol.toStringTag] = 'MediaKeyMessageEvent'; }; }
  if (typeof MediaKeySession === 'undefined') { window.MediaKeySession = function() { this[Symbol.toStringTag] = 'MediaKeySession'; }; }
  if (typeof MediaKeyStatusMap === 'undefined') { window.MediaKeyStatusMap = function() { this[Symbol.toStringTag] = 'MediaKeyStatusMap'; }; }
  if (typeof MediaKeySystemAccess === 'undefined') { window.MediaKeySystemAccess = function() { this[Symbol.toStringTag] = 'MediaKeySystemAccess'; }; }
  if (typeof MediaKeys === 'undefined') { window.MediaKeys = function() { this[Symbol.toStringTag] = 'MediaKeys'; }; }
  if (typeof MediaList === 'undefined') { window.MediaList = function() { this[Symbol.toStringTag] = 'MediaList'; }; }
  if (typeof MediaMetadata === 'undefined') { window.MediaMetadata = function() { this[Symbol.toStringTag] = 'MediaMetadata'; }; }
  if (typeof MediaQueryList === 'undefined') { window.MediaQueryList = function() { this[Symbol.toStringTag] = 'MediaQueryList'; }; }
  if (typeof MediaQueryListEvent === 'undefined') { window.MediaQueryListEvent = function() { this[Symbol.toStringTag] = 'MediaQueryListEvent'; }; }
  if (typeof MediaRecorder === 'undefined') { window.MediaRecorder = function() { this[Symbol.toStringTag] = 'MediaRecorder'; }; }
  if (typeof MediaSession === 'undefined') { window.MediaSession = function() { this[Symbol.toStringTag] = 'MediaSession'; }; }
  if (typeof MediaSource === 'undefined') { window.MediaSource = function() { this[Symbol.toStringTag] = 'MediaSource'; }; }
  if (typeof MediaSourceHandle === 'undefined') { window.MediaSourceHandle = function() { this[Symbol.toStringTag] = 'MediaSourceHandle'; }; }
  if (typeof MediaStream === 'undefined') { window.MediaStream = function() { this[Symbol.toStringTag] = 'MediaStream'; }; }
  if (typeof MediaStreamAudioDestinationNode === 'undefined') { window.MediaStreamAudioDestinationNode = function() { this[Symbol.toStringTag] = 'MediaStreamAudioDestinationNode'; }; }
  if (typeof MediaStreamAudioSourceNode === 'undefined') { window.MediaStreamAudioSourceNode = function() { this[Symbol.toStringTag] = 'MediaStreamAudioSourceNode'; }; }
  if (typeof MediaStreamEvent === 'undefined') { window.MediaStreamEvent = function() { this[Symbol.toStringTag] = 'MediaStreamEvent'; }; }
  if (typeof MediaStreamTrack === 'undefined') { window.MediaStreamTrack = function() { this[Symbol.toStringTag] = 'MediaStreamTrack'; }; }
  if (typeof MediaStreamTrackAudioStats === 'undefined') { window.MediaStreamTrackAudioStats = function() { this[Symbol.toStringTag] = 'MediaStreamTrackAudioStats'; }; }
  if (typeof MediaStreamTrackEvent === 'undefined') { window.MediaStreamTrackEvent = function() { this[Symbol.toStringTag] = 'MediaStreamTrackEvent'; }; }
  if (typeof MediaStreamTrackGenerator === 'undefined') { window.MediaStreamTrackGenerator = function() { this[Symbol.toStringTag] = 'MediaStreamTrackGenerator'; }; }
  if (typeof MediaStreamTrackProcessor === 'undefined') { window.MediaStreamTrackProcessor = function() { this[Symbol.toStringTag] = 'MediaStreamTrackProcessor'; }; }
  if (typeof MediaStreamTrackVideoStats === 'undefined') { window.MediaStreamTrackVideoStats = function() { this[Symbol.toStringTag] = 'MediaStreamTrackVideoStats'; }; }
  if (typeof MimeType === 'undefined') { window.MimeType = function() { this[Symbol.toStringTag] = 'MimeType'; }; }
  if (typeof MouseEvent === 'undefined') { window.MouseEvent = function() { this[Symbol.toStringTag] = 'MouseEvent'; }; }
  if (typeof MutationRecord === 'undefined') { window.MutationRecord = function() { this[Symbol.toStringTag] = 'MutationRecord'; }; }
  if (typeof NamedNodeMap === 'undefined') { window.NamedNodeMap = function() { this[Symbol.toStringTag] = 'NamedNodeMap'; }; }
  if (typeof NavigateEvent === 'undefined') { window.NavigateEvent = function() { this[Symbol.toStringTag] = 'NavigateEvent'; }; }
  if (typeof Navigation === 'undefined') { window.Navigation = function() { this[Symbol.toStringTag] = 'Navigation'; }; }
  if (typeof NavigationActivation === 'undefined') { window.NavigationActivation = function() { this[Symbol.toStringTag] = 'NavigationActivation'; }; }
  if (typeof NavigationCurrentEntryChangeEvent === 'undefined') { window.NavigationCurrentEntryChangeEvent = function() { this[Symbol.toStringTag] = 'NavigationCurrentEntryChangeEvent'; }; }
  if (typeof NavigationDestination === 'undefined') { window.NavigationDestination = function() { this[Symbol.toStringTag] = 'NavigationDestination'; }; }
  if (typeof NavigationHistoryEntry === 'undefined') { window.NavigationHistoryEntry = function() { this[Symbol.toStringTag] = 'NavigationHistoryEntry'; }; }
  if (typeof NavigationPrecommitController === 'undefined') { window.NavigationPrecommitController = function() { this[Symbol.toStringTag] = 'NavigationPrecommitController'; }; }
  if (typeof Navigation === 'undefined') { window.Navigation = function() { this[Symbol.toStringTag] = 'Navigation'; }; }
  if (typeof NavigationTransition === 'undefined') { window.NavigationTransition = function() { this[Symbol.toStringTag] = 'NavigationTransition'; }; }
  if (typeof NavigatorLogin === 'undefined') { window.NavigatorLogin = function() { this[Symbol.toStringTag] = 'NavigatorLogin'; }; }
  if (typeof NavigatorManagedData === 'undefined') { window.NavigatorManagedData = function() { this[Symbol.toStringTag] = 'NavigatorManagedData'; }; }
  if (typeof NavigatorUAData === 'undefined') { window.NavigatorUAData = function() { this[Symbol.toStringTag] = 'NavigatorUAData'; }; }
  if (typeof NetworkInformation === 'undefined') { window.NetworkInformation = function() { this[Symbol.toStringTag] = 'NetworkInformation'; }; }
  if (typeof NodeIterator === 'undefined') { window.NodeIterator = function() { this[Symbol.toStringTag] = 'NodeIterator'; }; }
  if (typeof NotRestoredReasonDetails === 'undefined') { window.NotRestoredReasonDetails = function() { this[Symbol.toStringTag] = 'NotRestoredReasonDetails'; }; }
  if (typeof NotRestoredReasons === 'undefined') { window.NotRestoredReasons = function() { this[Symbol.toStringTag] = 'NotRestoredReasons'; }; }
  if (typeof OTPCredential === 'undefined') { window.OTPCredential = function() { this[Symbol.toStringTag] = 'OTPCredential'; }; }
  if (typeof Observable === 'undefined') { window.Observable = function() { this[Symbol.toStringTag] = 'Observable'; }; }
  if (typeof OfflineAudioCompletionEvent === 'undefined') { window.OfflineAudioCompletionEvent = function() { this[Symbol.toStringTag] = 'OfflineAudioCompletionEvent'; }; }
  if (typeof OffscreenCanvas === 'undefined') { window.OffscreenCanvas = function() { this[Symbol.toStringTag] = 'OffscreenCanvas'; }; }
  if (typeof OffscreenCanvasRenderingContext2D === 'undefined') { window.OffscreenCanvasRenderingContext2D = function() { this[Symbol.toStringTag] = 'OffscreenCanvasRenderingContext2D'; }; }
  if (typeof OrientationSensor === 'undefined') { window.OrientationSensor = function() { this[Symbol.toStringTag] = 'OrientationSensor'; }; }
  if (typeof Origin === 'undefined') { window.Origin = function() { this[Symbol.toStringTag] = 'Origin'; }; }
  if (typeof OscillatorNode === 'undefined') { window.OscillatorNode = function() { this[Symbol.toStringTag] = 'OscillatorNode'; }; }
  if (typeof OverconstrainedError === 'undefined') { window.OverconstrainedError = function() { this[Symbol.toStringTag] = 'OverconstrainedError'; }; }
  if (typeof PageRevealEvent === 'undefined') { window.PageRevealEvent = function() { this[Symbol.toStringTag] = 'PageRevealEvent'; }; }
  if (typeof PageSwapEvent === 'undefined') { window.PageSwapEvent = function() { this[Symbol.toStringTag] = 'PageSwapEvent'; }; }
  if (typeof PageTransitionEvent === 'undefined') { window.PageTransitionEvent = function() { this[Symbol.toStringTag] = 'PageTransitionEvent'; }; }
  if (typeof PannerNode === 'undefined') { window.PannerNode = function() { this[Symbol.toStringTag] = 'PannerNode'; }; }
  if (typeof PasswordCredential === 'undefined') { window.PasswordCredential = function() { this[Symbol.toStringTag] = 'PasswordCredential'; }; }
  if (typeof PaymentAddress === 'undefined') { window.PaymentAddress = function() { this[Symbol.toStringTag] = 'PaymentAddress'; }; }
  if (typeof PaymentManager === 'undefined') { window.PaymentManager = function() { this[Symbol.toStringTag] = 'PaymentManager'; }; }
  if (typeof PaymentMethodChangeEvent === 'undefined') { window.PaymentMethodChangeEvent = function() { this[Symbol.toStringTag] = 'PaymentMethodChangeEvent'; }; }
  if (typeof PaymentRequest === 'undefined') { window.PaymentRequest = function() { this[Symbol.toStringTag] = 'PaymentRequest'; }; }
  if (typeof PaymentRequestUpdateEvent === 'undefined') { window.PaymentRequestUpdateEvent = function() { this[Symbol.toStringTag] = 'PaymentRequestUpdateEvent'; }; }
  if (typeof PaymentResponse === 'undefined') { window.PaymentResponse = function() { this[Symbol.toStringTag] = 'PaymentResponse'; }; }
  if (typeof PerformanceElementTiming === 'undefined') { window.PerformanceElementTiming = function() { this[Symbol.toStringTag] = 'PerformanceElementTiming'; }; }
  if (typeof PerformanceEntry === 'undefined') { window.PerformanceEntry = function() { this[Symbol.toStringTag] = 'PerformanceEntry'; }; }
  if (typeof PerformanceEventTiming === 'undefined') { window.PerformanceEventTiming = function() { this[Symbol.toStringTag] = 'PerformanceEventTiming'; }; }
  if (typeof PerformanceLongAnimationFrameTiming === 'undefined') { window.PerformanceLongAnimationFrameTiming = function() { this[Symbol.toStringTag] = 'PerformanceLongAnimationFrameTiming'; }; }
  if (typeof PerformanceLongTaskTiming === 'undefined') { window.PerformanceLongTaskTiming = function() { this[Symbol.toStringTag] = 'PerformanceLongTaskTiming'; }; }
  if (typeof PerformanceMark === 'undefined') { window.PerformanceMark = function() { this[Symbol.toStringTag] = 'PerformanceMark'; }; }
  if (typeof PerformanceMeasure === 'undefined') { window.PerformanceMeasure = function() { this[Symbol.toStringTag] = 'PerformanceMeasure'; }; }
  if (typeof PerformanceNavigation === 'undefined') { window.PerformanceNavigation = function() { this[Symbol.toStringTag] = 'PerformanceNavigation'; }; }
  if (typeof PerformanceNavigationTiming === 'undefined') { window.PerformanceNavigationTiming = function() { this[Symbol.toStringTag] = 'PerformanceNavigationTiming'; }; }
  if (typeof PerformanceObserver === 'undefined') { window.PerformanceObserver = function() { this[Symbol.toStringTag] = 'PerformanceObserver'; }; }
  if (typeof PerformanceObserverEntryList === 'undefined') { window.PerformanceObserverEntryList = function() { this[Symbol.toStringTag] = 'PerformanceObserverEntryList'; }; }
  if (typeof PerformancePaintTiming === 'undefined') { window.PerformancePaintTiming = function() { this[Symbol.toStringTag] = 'PerformancePaintTiming'; }; }
  if (typeof PerformanceResourceTiming === 'undefined') { window.PerformanceResourceTiming = function() { this[Symbol.toStringTag] = 'PerformanceResourceTiming'; }; }
  if (typeof PerformanceScriptTiming === 'undefined') { window.PerformanceScriptTiming = function() { this[Symbol.toStringTag] = 'PerformanceScriptTiming'; }; }
  if (typeof PerformanceServerTiming === 'undefined') { window.PerformanceServerTiming = function() { this[Symbol.toStringTag] = 'PerformanceServerTiming'; }; }
  if (typeof PerformanceTiming === 'undefined') { window.PerformanceTiming = function() { this[Symbol.toStringTag] = 'PerformanceTiming'; }; }
  if (typeof PerformanceTimingConfidence === 'undefined') { window.PerformanceTimingConfidence = function() { this[Symbol.toStringTag] = 'PerformanceTimingConfidence'; }; }
  if (typeof PeriodicSyncManager === 'undefined') { window.PeriodicSyncManager = function() { this[Symbol.toStringTag] = 'PeriodicSyncManager'; }; }
  if (typeof PeriodicWave === 'undefined') { window.PeriodicWave = function() { this[Symbol.toStringTag] = 'PeriodicWave'; }; }
  if (typeof PermissionStatus === 'undefined') { window.PermissionStatus = function() { this[Symbol.toStringTag] = 'PermissionStatus'; }; }
  if (typeof Permissions === 'undefined') { window.Permissions = function() { this[Symbol.toStringTag] = 'Permissions'; }; }
  if (typeof PictureInPictureEvent === 'undefined') { window.PictureInPictureEvent = function() { this[Symbol.toStringTag] = 'PictureInPictureEvent'; }; }
  if (typeof PictureInPictureWindow === 'undefined') { window.PictureInPictureWindow = function() { this[Symbol.toStringTag] = 'PictureInPictureWindow'; }; }
  if (typeof Plugin === 'undefined') { window.Plugin = function() { this[Symbol.toStringTag] = 'Plugin'; }; }
  if (typeof PointerEvent === 'undefined') { window.PointerEvent = function() { this[Symbol.toStringTag] = 'PointerEvent'; }; }
  if (typeof PopStateEvent === 'undefined') { window.PopStateEvent = function() { this[Symbol.toStringTag] = 'PopStateEvent'; }; }
  if (typeof Presentation === 'undefined') { window.Presentation = function() { this[Symbol.toStringTag] = 'Presentation'; }; }
  if (typeof PresentationAvailability === 'undefined') { window.PresentationAvailability = function() { this[Symbol.toStringTag] = 'PresentationAvailability'; }; }
  if (typeof PresentationConnection === 'undefined') { window.PresentationConnection = function() { this[Symbol.toStringTag] = 'PresentationConnection'; }; }
  if (typeof PresentationConnectionAvailableEvent === 'undefined') { window.PresentationConnectionAvailableEvent = function() { this[Symbol.toStringTag] = 'PresentationConnectionAvailableEvent'; }; }
  if (typeof PresentationConnectionCloseEvent === 'undefined') { window.PresentationConnectionCloseEvent = function() { this[Symbol.toStringTag] = 'PresentationConnectionCloseEvent'; }; }
  if (typeof PresentationConnectionList === 'undefined') { window.PresentationConnectionList = function() { this[Symbol.toStringTag] = 'PresentationConnectionList'; }; }
  if (typeof PresentationReceiver === 'undefined') { window.PresentationReceiver = function() { this[Symbol.toStringTag] = 'PresentationReceiver'; }; }
  if (typeof PresentationRequest === 'undefined') { window.PresentationRequest = function() { this[Symbol.toStringTag] = 'PresentationRequest'; }; }
  if (typeof PressureObserver === 'undefined') { window.PressureObserver = function() { this[Symbol.toStringTag] = 'PressureObserver'; }; }
  if (typeof PressureRecord === 'undefined') { window.PressureRecord = function() { this[Symbol.toStringTag] = 'PressureRecord'; }; }
  if (typeof ProcessingInstruction === 'undefined') { window.ProcessingInstruction = function() { this[Symbol.toStringTag] = 'ProcessingInstruction'; }; }
  if (typeof Profiler === 'undefined') { window.Profiler = function() { this[Symbol.toStringTag] = 'Profiler'; }; }
  if (typeof ProtectedAudience === 'undefined') { window.ProtectedAudience = function() { this[Symbol.toStringTag] = 'ProtectedAudience'; }; }
  if (typeof PublicKeyCredential === 'undefined') { window.PublicKeyCredential = function() { this[Symbol.toStringTag] = 'PublicKeyCredential'; }; }
  if (typeof PushManager === 'undefined') { window.PushManager = function() { this[Symbol.toStringTag] = 'PushManager'; }; }
  if (typeof PushSubscription === 'undefined') { window.PushSubscription = function() { this[Symbol.toStringTag] = 'PushSubscription'; }; }
  if (typeof PushSubscriptionOptions === 'undefined') { window.PushSubscriptionOptions = function() { this[Symbol.toStringTag] = 'PushSubscriptionOptions'; }; }
  if (typeof QuotaExceededError === 'undefined') { window.QuotaExceededError = function() { this[Symbol.toStringTag] = 'QuotaExceededError'; }; }
  if (typeof RTCCertificate === 'undefined') { window.RTCCertificate = function() { this[Symbol.toStringTag] = 'RTCCertificate'; }; }
  if (typeof RTCDTMFSender === 'undefined') { window.RTCDTMFSender = function() { this[Symbol.toStringTag] = 'RTCDTMFSender'; }; }
  if (typeof RTCDTMFToneChangeEvent === 'undefined') { window.RTCDTMFToneChangeEvent = function() { this[Symbol.toStringTag] = 'RTCDTMFToneChangeEvent'; }; }
  if (typeof RTCDataChannel === 'undefined') { window.RTCDataChannel = function() { this[Symbol.toStringTag] = 'RTCDataChannel'; }; }
  if (typeof RTCDataChannelEvent === 'undefined') { window.RTCDataChannelEvent = function() { this[Symbol.toStringTag] = 'RTCDataChannelEvent'; }; }
  if (typeof RTCDtlsTransport === 'undefined') { window.RTCDtlsTransport = function() { this[Symbol.toStringTag] = 'RTCDtlsTransport'; }; }
  if (typeof RTCEncodedAudioFrame === 'undefined') { window.RTCEncodedAudioFrame = function() { this[Symbol.toStringTag] = 'RTCEncodedAudioFrame'; }; }
  if (typeof RTCEncodedVideoFrame === 'undefined') { window.RTCEncodedVideoFrame = function() { this[Symbol.toStringTag] = 'RTCEncodedVideoFrame'; }; }
  if (typeof RTCError === 'undefined') { window.RTCError = function() { this[Symbol.toStringTag] = 'RTCError'; }; }
  if (typeof RTCErrorEvent === 'undefined') { window.RTCErrorEvent = function() { this[Symbol.toStringTag] = 'RTCErrorEvent'; }; }
  if (typeof RTCIceCandidate === 'undefined') { window.RTCIceCandidate = function() { this[Symbol.toStringTag] = 'RTCIceCandidate'; }; }
  if (typeof RTCIceTransport === 'undefined') { window.RTCIceTransport = function() { this[Symbol.toStringTag] = 'RTCIceTransport'; }; }
  if (typeof RTCPeerConnectionIceErrorEvent === 'undefined') { window.RTCPeerConnectionIceErrorEvent = function() { this[Symbol.toStringTag] = 'RTCPeerConnectionIceErrorEvent'; }; }
  if (typeof RTCPeerConnectionIceEvent === 'undefined') { window.RTCPeerConnectionIceEvent = function() { this[Symbol.toStringTag] = 'RTCPeerConnectionIceEvent'; }; }
  if (typeof RTCRtpReceiver === 'undefined') { window.RTCRtpReceiver = function() { this[Symbol.toStringTag] = 'RTCRtpReceiver'; }; }
  if (typeof RTCRtpScriptTransform === 'undefined') { window.RTCRtpScriptTransform = function() { this[Symbol.toStringTag] = 'RTCRtpScriptTransform'; }; }
  if (typeof RTCRtpSender === 'undefined') { window.RTCRtpSender = function() { this[Symbol.toStringTag] = 'RTCRtpSender'; }; }
  if (typeof RTCRtpTransceiver === 'undefined') { window.RTCRtpTransceiver = function() { this[Symbol.toStringTag] = 'RTCRtpTransceiver'; }; }
  if (typeof RTCSctpTransport === 'undefined') { window.RTCSctpTransport = function() { this[Symbol.toStringTag] = 'RTCSctpTransport'; }; }
  if (typeof RTCSessionDescription === 'undefined') { window.RTCSessionDescription = function() { this[Symbol.toStringTag] = 'RTCSessionDescription'; }; }
  if (typeof RTCStatsReport === 'undefined') { window.RTCStatsReport = function() { this[Symbol.toStringTag] = 'RTCStatsReport'; }; }
  if (typeof RTCTrackEvent === 'undefined') { window.RTCTrackEvent = function() { this[Symbol.toStringTag] = 'RTCTrackEvent'; }; }
  if (typeof RadioNodeList === 'undefined') { window.RadioNodeList = function() { this[Symbol.toStringTag] = 'RadioNodeList'; }; }
  if (typeof Range === 'undefined') { window.Range = function() { this[Symbol.toStringTag] = 'Range'; }; }
  if (typeof ReadableByteStreamController === 'undefined') { window.ReadableByteStreamController = function() { this[Symbol.toStringTag] = 'ReadableByteStreamController'; }; }
  if (typeof ReadableStreamBYOBReader === 'undefined') { window.ReadableStreamBYOBReader = function() { this[Symbol.toStringTag] = 'ReadableStreamBYOBReader'; }; }
  if (typeof ReadableStreamBYOBRequest === 'undefined') { window.ReadableStreamBYOBRequest = function() { this[Symbol.toStringTag] = 'ReadableStreamBYOBRequest'; }; }
  if (typeof ReadableStreamDefaultController === 'undefined') { window.ReadableStreamDefaultController = function() { this[Symbol.toStringTag] = 'ReadableStreamDefaultController'; }; }
  if (typeof ReadableStreamDefaultReader === 'undefined') { window.ReadableStreamDefaultReader = function() { this[Symbol.toStringTag] = 'ReadableStreamDefaultReader'; }; }
  if (typeof RelativeOrientationSensor === 'undefined') { window.RelativeOrientationSensor = function() { this[Symbol.toStringTag] = 'RelativeOrientationSensor'; }; }
  if (typeof RemotePlayback === 'undefined') { window.RemotePlayback = function() { this[Symbol.toStringTag] = 'RemotePlayback'; }; }
  if (typeof ReportBody === 'undefined') { window.ReportBody = function() { this[Symbol.toStringTag] = 'ReportBody'; }; }
  if (typeof ReportingObserver === 'undefined') { window.ReportingObserver = function() { this[Symbol.toStringTag] = 'ReportingObserver'; }; }
  if (typeof ResizeObserverEntry === 'undefined') { window.ResizeObserverEntry = function() { this[Symbol.toStringTag] = 'ResizeObserverEntry'; }; }
  if (typeof ResizeObserverSize === 'undefined') { window.ResizeObserverSize = function() { this[Symbol.toStringTag] = 'ResizeObserverSize'; }; }
  if (typeof RestrictionTarget === 'undefined') { window.RestrictionTarget = function() { this[Symbol.toStringTag] = 'RestrictionTarget'; }; }
  if (typeof SMS === 'undefined') { window.SMS = function() { this[Symbol.toStringTag] = 'SMS'; }; }
  if (typeof SVGAElement === 'undefined') { window.SVGAElement = function() { this[Symbol.toStringTag] = 'SVGAElement'; }; }
  if (typeof SVGAngle === 'undefined') { window.SVGAngle = function() { this[Symbol.toStringTag] = 'SVGAngle'; }; }
  if (typeof SVGAnimateElement === 'undefined') { window.SVGAnimateElement = function() { this[Symbol.toStringTag] = 'SVGAnimateElement'; }; }
  if (typeof SVGAnimateMotionElement === 'undefined') { window.SVGAnimateMotionElement = function() { this[Symbol.toStringTag] = 'SVGAnimateMotionElement'; }; }
  if (typeof SVGAnimateTransformElement === 'undefined') { window.SVGAnimateTransformElement = function() { this[Symbol.toStringTag] = 'SVGAnimateTransformElement'; }; }
  if (typeof SVGAnimatedAngle === 'undefined') { window.SVGAnimatedAngle = function() { this[Symbol.toStringTag] = 'SVGAnimatedAngle'; }; }
  if (typeof SVGAnimatedBoolean === 'undefined') { window.SVGAnimatedBoolean = function() { this[Symbol.toStringTag] = 'SVGAnimatedBoolean'; }; }
  if (typeof SVGAnimatedEnumeration === 'undefined') { window.SVGAnimatedEnumeration = function() { this[Symbol.toStringTag] = 'SVGAnimatedEnumeration'; }; }
  if (typeof SVGAnimatedInteger === 'undefined') { window.SVGAnimatedInteger = function() { this[Symbol.toStringTag] = 'SVGAnimatedInteger'; }; }
  if (typeof SVGAnimatedLength === 'undefined') { window.SVGAnimatedLength = function() { this[Symbol.toStringTag] = 'SVGAnimatedLength'; }; }
  if (typeof SVGAnimatedLengthList === 'undefined') { window.SVGAnimatedLengthList = function() { this[Symbol.toStringTag] = 'SVGAnimatedLengthList'; }; }
  if (typeof SVGAnimatedNumber === 'undefined') { window.SVGAnimatedNumber = function() { this[Symbol.toStringTag] = 'SVGAnimatedNumber'; }; }
  if (typeof SVGAnimatedNumberList === 'undefined') { window.SVGAnimatedNumberList = function() { this[Symbol.toStringTag] = 'SVGAnimatedNumberList'; }; }
  if (typeof SVGAnimatedPreserveAspectRatio === 'undefined') { window.SVGAnimatedPreserveAspectRatio = function() { this[Symbol.toStringTag] = 'SVGAnimatedPreserveAspectRatio'; }; }
  if (typeof SVGAnimatedRect === 'undefined') { window.SVGAnimatedRect = function() { this[Symbol.toStringTag] = 'SVGAnimatedRect'; }; }
  if (typeof SVGAnimatedString === 'undefined') { window.SVGAnimatedString = function() { this[Symbol.toStringTag] = 'SVGAnimatedString'; }; }
  if (typeof SVGAnimatedTransformList === 'undefined') { window.SVGAnimatedTransformList = function() { this[Symbol.toStringTag] = 'SVGAnimatedTransformList'; }; }
  if (typeof SVGAnimationElement === 'undefined') { window.SVGAnimationElement = function() { this[Symbol.toStringTag] = 'SVGAnimationElement'; }; }
  if (typeof SVGCircleElement === 'undefined') { window.SVGCircleElement = function() { this[Symbol.toStringTag] = 'SVGCircleElement'; }; }
  if (typeof SVGClipPathElement === 'undefined') { window.SVGClipPathElement = function() { this[Symbol.toStringTag] = 'SVGClipPathElement'; }; }
  if (typeof SVGComponentTransferFunctionElement === 'undefined') { window.SVGComponentTransferFunctionElement = function() { this[Symbol.toStringTag] = 'SVGComponentTransferFunctionElement'; }; }
  if (typeof SVGDefsElement === 'undefined') { window.SVGDefsElement = function() { this[Symbol.toStringTag] = 'SVGDefsElement'; }; }
  if (typeof SVGDescElement === 'undefined') { window.SVGDescElement = function() { this[Symbol.toStringTag] = 'SVGDescElement'; }; }
  if (typeof SVGElement === 'undefined') { window.SVGElement = function() { this[Symbol.toStringTag] = 'SVGElement'; }; }
  if (typeof SVGEllipseElement === 'undefined') { window.SVGEllipseElement = function() { this[Symbol.toStringTag] = 'SVGEllipseElement'; }; }
  if (typeof SVGFEBlendElement === 'undefined') { window.SVGFEBlendElement = function() { this[Symbol.toStringTag] = 'SVGFEBlendElement'; }; }
  if (typeof SVGFEColorMatrixElement === 'undefined') { window.SVGFEColorMatrixElement = function() { this[Symbol.toStringTag] = 'SVGFEColorMatrixElement'; }; }
  if (typeof SVGFEComponentTransferElement === 'undefined') { window.SVGFEComponentTransferElement = function() { this[Symbol.toStringTag] = 'SVGFEComponentTransferElement'; }; }
  if (typeof SVGFECompositeElement === 'undefined') { window.SVGFECompositeElement = function() { this[Symbol.toStringTag] = 'SVGFECompositeElement'; }; }
  if (typeof SVGFEConvolveMatrixElement === 'undefined') { window.SVGFEConvolveMatrixElement = function() { this[Symbol.toStringTag] = 'SVGFEConvolveMatrixElement'; }; }
  if (typeof SVGFEDiffuseLightingElement === 'undefined') { window.SVGFEDiffuseLightingElement = function() { this[Symbol.toStringTag] = 'SVGFEDiffuseLightingElement'; }; }
  if (typeof SVGFEDisplacementMapElement === 'undefined') { window.SVGFEDisplacementMapElement = function() { this[Symbol.toStringTag] = 'SVGFEDisplacementMapElement'; }; }
  if (typeof SVGFEDistantLightElement === 'undefined') { window.SVGFEDistantLightElement = function() { this[Symbol.toStringTag] = 'SVGFEDistantLightElement'; }; }
  if (typeof SVGFEDropShadowElement === 'undefined') { window.SVGFEDropShadowElement = function() { this[Symbol.toStringTag] = 'SVGFEDropShadowElement'; }; }
  if (typeof SVGFEFloodElement === 'undefined') { window.SVGFEFloodElement = function() { this[Symbol.toStringTag] = 'SVGFEFloodElement'; }; }
  if (typeof SVGFEFuncAElement === 'undefined') { window.SVGFEFuncAElement = function() { this[Symbol.toStringTag] = 'SVGFEFuncAElement'; }; }
  if (typeof SVGFEFuncBElement === 'undefined') { window.SVGFEFuncBElement = function() { this[Symbol.toStringTag] = 'SVGFEFuncBElement'; }; }
  if (typeof SVGFEFuncGElement === 'undefined') { window.SVGFEFuncGElement = function() { this[Symbol.toStringTag] = 'SVGFEFuncGElement'; }; }
  if (typeof SVGFEFuncRElement === 'undefined') { window.SVGFEFuncRElement = function() { this[Symbol.toStringTag] = 'SVGFEFuncRElement'; }; }
  if (typeof SVGFEGaussianBlurElement === 'undefined') { window.SVGFEGaussianBlurElement = function() { this[Symbol.toStringTag] = 'SVGFEGaussianBlurElement'; }; }
  if (typeof SVGFEImageElement === 'undefined') { window.SVGFEImageElement = function() { this[Symbol.toStringTag] = 'SVGFEImageElement'; }; }
  if (typeof SVGFEMergeElement === 'undefined') { window.SVGFEMergeElement = function() { this[Symbol.toStringTag] = 'SVGFEMergeElement'; }; }
  if (typeof SVGFEMergeNodeElement === 'undefined') { window.SVGFEMergeNodeElement = function() { this[Symbol.toStringTag] = 'SVGFEMergeNodeElement'; }; }
  if (typeof SVGFEMorphologyElement === 'undefined') { window.SVGFEMorphologyElement = function() { this[Symbol.toStringTag] = 'SVGFEMorphologyElement'; }; }
  if (typeof SVGFEOffsetElement === 'undefined') { window.SVGFEOffsetElement = function() { this[Symbol.toStringTag] = 'SVGFEOffsetElement'; }; }
  if (typeof SVGFEPointLightElement === 'undefined') { window.SVGFEPointLightElement = function() { this[Symbol.toStringTag] = 'SVGFEPointLightElement'; }; }
  if (typeof SVGFESpecularLightingElement === 'undefined') { window.SVGFESpecularLightingElement = function() { this[Symbol.toStringTag] = 'SVGFESpecularLightingElement'; }; }
  if (typeof SVGFESpotLightElement === 'undefined') { window.SVGFESpotLightElement = function() { this[Symbol.toStringTag] = 'SVGFESpotLightElement'; }; }
  if (typeof SVGFETileElement === 'undefined') { window.SVGFETileElement = function() { this[Symbol.toStringTag] = 'SVGFETileElement'; }; }
  if (typeof SVGFETurbulenceElement === 'undefined') { window.SVGFETurbulenceElement = function() { this[Symbol.toStringTag] = 'SVGFETurbulenceElement'; }; }
  if (typeof SVGFilterElement === 'undefined') { window.SVGFilterElement = function() { this[Symbol.toStringTag] = 'SVGFilterElement'; }; }
  if (typeof SVGForeignObjectElement === 'undefined') { window.SVGForeignObjectElement = function() { this[Symbol.toStringTag] = 'SVGForeignObjectElement'; }; }
  if (typeof SVGGElement === 'undefined') { window.SVGGElement = function() { this[Symbol.toStringTag] = 'SVGGElement'; }; }
  if (typeof SVGGeometryElement === 'undefined') { window.SVGGeometryElement = function() { this[Symbol.toStringTag] = 'SVGGeometryElement'; }; }
  if (typeof SVGGradientElement === 'undefined') { window.SVGGradientElement = function() { this[Symbol.toStringTag] = 'SVGGradientElement'; }; }
  if (typeof SVGGraphicsElement === 'undefined') { window.SVGGraphicsElement = function() { this[Symbol.toStringTag] = 'SVGGraphicsElement'; }; }
  if (typeof SVGImageElement === 'undefined') { window.SVGImageElement = function() { this[Symbol.toStringTag] = 'SVGImageElement'; }; }
  if (typeof SVGLength === 'undefined') { window.SVGLength = function() { this[Symbol.toStringTag] = 'SVGLength'; }; }
  if (typeof SVGLengthList === 'undefined') { window.SVGLengthList = function() { this[Symbol.toStringTag] = 'SVGLengthList'; }; }
  if (typeof SVGLineElement === 'undefined') { window.SVGLineElement = function() { this[Symbol.toStringTag] = 'SVGLineElement'; }; }
  if (typeof SVGLinearGradientElement === 'undefined') { window.SVGLinearGradientElement = function() { this[Symbol.toStringTag] = 'SVGLinearGradientElement'; }; }
  if (typeof SVGMPathElement === 'undefined') { window.SVGMPathElement = function() { this[Symbol.toStringTag] = 'SVGMPathElement'; }; }
  if (typeof SVGMarkerElement === 'undefined') { window.SVGMarkerElement = function() { this[Symbol.toStringTag] = 'SVGMarkerElement'; }; }
  if (typeof SVGMaskElement === 'undefined') { window.SVGMaskElement = function() { this[Symbol.toStringTag] = 'SVGMaskElement'; }; }
  if (typeof SVGMatrix === 'undefined') { window.SVGMatrix = function() { this[Symbol.toStringTag] = 'SVGMatrix'; }; }
  if (typeof SVGMetadataElement === 'undefined') { window.SVGMetadataElement = function() { this[Symbol.toStringTag] = 'SVGMetadataElement'; }; }
  if (typeof SVGNumber === 'undefined') { window.SVGNumber = function() { this[Symbol.toStringTag] = 'SVGNumber'; }; }
  if (typeof SVGNumberList === 'undefined') { window.SVGNumberList = function() { this[Symbol.toStringTag] = 'SVGNumberList'; }; }
  if (typeof SVGPathElement === 'undefined') { window.SVGPathElement = function() { this[Symbol.toStringTag] = 'SVGPathElement'; }; }
  if (typeof SVGPatternElement === 'undefined') { window.SVGPatternElement = function() { this[Symbol.toStringTag] = 'SVGPatternElement'; }; }
  if (typeof SVGPoint === 'undefined') { window.SVGPoint = function() { this[Symbol.toStringTag] = 'SVGPoint'; }; }
  if (typeof SVGPointList === 'undefined') { window.SVGPointList = function() { this[Symbol.toStringTag] = 'SVGPointList'; }; }
  if (typeof SVGPolygonElement === 'undefined') { window.SVGPolygonElement = function() { this[Symbol.toStringTag] = 'SVGPolygonElement'; }; }
  if (typeof SVGPolylineElement === 'undefined') { window.SVGPolylineElement = function() { this[Symbol.toStringTag] = 'SVGPolylineElement'; }; }
  if (typeof SVGPreserveAspectRatio === 'undefined') { window.SVGPreserveAspectRatio = function() { this[Symbol.toStringTag] = 'SVGPreserveAspectRatio'; }; }
  if (typeof SVGRadialGradientElement === 'undefined') { window.SVGRadialGradientElement = function() { this[Symbol.toStringTag] = 'SVGRadialGradientElement'; }; }
  if (typeof SVGRect === 'undefined') { window.SVGRect = function() { this[Symbol.toStringTag] = 'SVGRect'; }; }
  if (typeof SVGRectElement === 'undefined') { window.SVGRectElement = function() { this[Symbol.toStringTag] = 'SVGRectElement'; }; }
  if (typeof SVGSVGElement === 'undefined') { window.SVGSVGElement = function() { this[Symbol.toStringTag] = 'SVGSVGElement'; }; }
  if (typeof SVGScriptElement === 'undefined') { window.SVGScriptElement = function() { this[Symbol.toStringTag] = 'SVGScriptElement'; }; }
  if (typeof SVGSetElement === 'undefined') { window.SVGSetElement = function() { this[Symbol.toStringTag] = 'SVGSetElement'; }; }
  if (typeof SVGStopElement === 'undefined') { window.SVGStopElement = function() { this[Symbol.toStringTag] = 'SVGStopElement'; }; }
  if (typeof SVGStringList === 'undefined') { window.SVGStringList = function() { this[Symbol.toStringTag] = 'SVGStringList'; }; }
  if (typeof SVGStyleElement === 'undefined') { window.SVGStyleElement = function() { this[Symbol.toStringTag] = 'SVGStyleElement'; }; }
  if (typeof SVGSwitchElement === 'undefined') { window.SVGSwitchElement = function() { this[Symbol.toStringTag] = 'SVGSwitchElement'; }; }
  if (typeof SVGSymbolElement === 'undefined') { window.SVGSymbolElement = function() { this[Symbol.toStringTag] = 'SVGSymbolElement'; }; }
  if (typeof SVGTSpanElement === 'undefined') { window.SVGTSpanElement = function() { this[Symbol.toStringTag] = 'SVGTSpanElement'; }; }
  if (typeof SVGTextContentElement === 'undefined') { window.SVGTextContentElement = function() { this[Symbol.toStringTag] = 'SVGTextContentElement'; }; }
  if (typeof SVGTextElement === 'undefined') { window.SVGTextElement = function() { this[Symbol.toStringTag] = 'SVGTextElement'; }; }
  if (typeof SVGTextPathElement === 'undefined') { window.SVGTextPathElement = function() { this[Symbol.toStringTag] = 'SVGTextPathElement'; }; }
  if (typeof SVGTextPositioningElement === 'undefined') { window.SVGTextPositioningElement = function() { this[Symbol.toStringTag] = 'SVGTextPositioningElement'; }; }
  if (typeof SVGTitleElement === 'undefined') { window.SVGTitleElement = function() { this[Symbol.toStringTag] = 'SVGTitleElement'; }; }
  if (typeof SVGTransform === 'undefined') { window.SVGTransform = function() { this[Symbol.toStringTag] = 'SVGTransform'; }; }
  if (typeof SVGTransformList === 'undefined') { window.SVGTransformList = function() { this[Symbol.toStringTag] = 'SVGTransformList'; }; }
  if (typeof SVGUnitTypes === 'undefined') { window.SVGUnitTypes = function() { this[Symbol.toStringTag] = 'SVGUnitTypes'; }; }
  if (typeof SVGUseElement === 'undefined') { window.SVGUseElement = function() { this[Symbol.toStringTag] = 'SVGUseElement'; }; }
  if (typeof SVGViewElement === 'undefined') { window.SVGViewElement = function() { this[Symbol.toStringTag] = 'SVGViewElement'; }; }
  if (typeof Sanitizer === 'undefined') { window.Sanitizer = function() { this[Symbol.toStringTag] = 'Sanitizer'; }; }
  if (typeof Scheduler === 'undefined') { window.Scheduler = function() { this[Symbol.toStringTag] = 'Scheduler'; }; }
  if (typeof Scheduling === 'undefined') { window.Scheduling = function() { this[Symbol.toStringTag] = 'Scheduling'; }; }
  if (typeof ScreenDetailed === 'undefined') { window.ScreenDetailed = function() { this[Symbol.toStringTag] = 'ScreenDetailed'; }; }
  if (typeof ScreenDetails === 'undefined') { window.ScreenDetails = function() { this[Symbol.toStringTag] = 'ScreenDetails'; }; }
  if (typeof ScreenOrientation === 'undefined') { window.ScreenOrientation = function() { this[Symbol.toStringTag] = 'ScreenOrientation'; }; }
  if (typeof ScriptProcessorNode === 'undefined') { window.ScriptProcessorNode = function() { this[Symbol.toStringTag] = 'ScriptProcessorNode'; }; }
  if (typeof ScrollTimeline === 'undefined') { window.ScrollTimeline = function() { this[Symbol.toStringTag] = 'ScrollTimeline'; }; }
  if (typeof SecurityPolicyViolationEvent === 'undefined') { window.SecurityPolicyViolationEvent = function() { this[Symbol.toStringTag] = 'SecurityPolicyViolationEvent'; }; }
  if (typeof Selection === 'undefined') { window.Selection = function() { this[Symbol.toStringTag] = 'Selection'; }; }
  if (typeof Sensor === 'undefined') { window.Sensor = function() { this[Symbol.toStringTag] = 'Sensor'; }; }
  if (typeof SensorErrorEvent === 'undefined') { window.SensorErrorEvent = function() { this[Symbol.toStringTag] = 'SensorErrorEvent'; }; }
  if (typeof Serial === 'undefined') { window.Serial = function() { this[Symbol.toStringTag] = 'Serial'; }; }
  if (typeof SerialPort === 'undefined') { window.SerialPort = function() { this[Symbol.toStringTag] = 'SerialPort'; }; }
  if (typeof ServiceWorker === 'undefined') { window.ServiceWorker = function() { this[Symbol.toStringTag] = 'ServiceWorker'; }; }
  if (typeof ServiceWorkerContainer === 'undefined') { window.ServiceWorkerContainer = function() { this[Symbol.toStringTag] = 'ServiceWorkerContainer'; }; }
  if (typeof ServiceWorkerRegistration === 'undefined') { window.ServiceWorkerRegistration = function() { this[Symbol.toStringTag] = 'ServiceWorkerRegistration'; }; }
  if (typeof ShadowRoot === 'undefined') { window.ShadowRoot = function() { this[Symbol.toStringTag] = 'ShadowRoot'; }; }
  if (typeof SharedStorage === 'undefined') { window.SharedStorage = function() { this[Symbol.toStringTag] = 'SharedStorage'; }; }
  if (typeof SharedStorageAppendMethod === 'undefined') { window.SharedStorageAppendMethod = function() { this[Symbol.toStringTag] = 'SharedStorageAppendMethod'; }; }
  if (typeof SharedStorageClearMethod === 'undefined') { window.SharedStorageClearMethod = function() { this[Symbol.toStringTag] = 'SharedStorageClearMethod'; }; }
  if (typeof SharedStorageDeleteMethod === 'undefined') { window.SharedStorageDeleteMethod = function() { this[Symbol.toStringTag] = 'SharedStorageDeleteMethod'; }; }
  if (typeof SharedStorageModifierMethod === 'undefined') { window.SharedStorageModifierMethod = function() { this[Symbol.toStringTag] = 'SharedStorageModifierMethod'; }; }
  if (typeof SharedStorageSetMethod === 'undefined') { window.SharedStorageSetMethod = function() { this[Symbol.toStringTag] = 'SharedStorageSetMethod'; }; }
  if (typeof SharedStorageWorklet === 'undefined') { window.SharedStorageWorklet = function() { this[Symbol.toStringTag] = 'SharedStorageWorklet'; }; }
  if (typeof SnapEvent === 'undefined') { window.SnapEvent = function() { this[Symbol.toStringTag] = 'SnapEvent'; }; }
  if (typeof SourceBuffer === 'undefined') { window.SourceBuffer = function() { this[Symbol.toStringTag] = 'SourceBuffer'; }; }
  if (typeof SourceBufferList === 'undefined') { window.SourceBufferList = function() { this[Symbol.toStringTag] = 'SourceBufferList'; }; }
  if (typeof SpeechGrammar === 'undefined') { window.SpeechGrammar = function() { this[Symbol.toStringTag] = 'SpeechGrammar'; }; }
  if (typeof SpeechGrammarList === 'undefined') { window.SpeechGrammarList = function() { this[Symbol.toStringTag] = 'SpeechGrammarList'; }; }
  if (typeof SpeechRecognition === 'undefined') { window.SpeechRecognition = function() { this[Symbol.toStringTag] = 'SpeechRecognition'; }; }
  if (typeof SpeechRecognitionErrorEvent === 'undefined') { window.SpeechRecognitionErrorEvent = function() { this[Symbol.toStringTag] = 'SpeechRecognitionErrorEvent'; }; }
  if (typeof SpeechRecognitionEvent === 'undefined') { window.SpeechRecognitionEvent = function() { this[Symbol.toStringTag] = 'SpeechRecognitionEvent'; }; }
  if (typeof SpeechRecognitionPhrase === 'undefined') { window.SpeechRecognitionPhrase = function() { this[Symbol.toStringTag] = 'SpeechRecognitionPhrase'; }; }
  if (typeof SpeechSynthesis === 'undefined') { window.SpeechSynthesis = function() { this[Symbol.toStringTag] = 'SpeechSynthesis'; }; }
  if (typeof SpeechSynthesisErrorEvent === 'undefined') { window.SpeechSynthesisErrorEvent = function() { this[Symbol.toStringTag] = 'SpeechSynthesisErrorEvent'; }; }
  if (typeof SpeechSynthesisEvent === 'undefined') { window.SpeechSynthesisEvent = function() { this[Symbol.toStringTag] = 'SpeechSynthesisEvent'; }; }
  if (typeof SpeechSynthesisUtterance === 'undefined') { window.SpeechSynthesisUtterance = function() { this[Symbol.toStringTag] = 'SpeechSynthesisUtterance'; }; }
  if (typeof SpeechSynthesisVoice === 'undefined') { window.SpeechSynthesisVoice = function() { this[Symbol.toStringTag] = 'SpeechSynthesisVoice'; }; }
  if (typeof StaticRange === 'undefined') { window.StaticRange = function() { this[Symbol.toStringTag] = 'StaticRange'; }; }
  if (typeof StereoPannerNode === 'undefined') { window.StereoPannerNode = function() { this[Symbol.toStringTag] = 'StereoPannerNode'; }; }
  if (typeof StorageBucket === 'undefined') { window.StorageBucket = function() { this[Symbol.toStringTag] = 'StorageBucket'; }; }
  if (typeof StorageBucketManager === 'undefined') { window.StorageBucketManager = function() { this[Symbol.toStringTag] = 'StorageBucketManager'; }; }
  if (typeof StorageEvent === 'undefined') { window.StorageEvent = function() { this[Symbol.toStringTag] = 'StorageEvent'; }; }
  if (typeof StorageManager === 'undefined') { window.StorageManager = function() { this[Symbol.toStringTag] = 'StorageManager'; }; }
  if (typeof StylePropertyMap === 'undefined') { window.StylePropertyMap = function() { this[Symbol.toStringTag] = 'StylePropertyMap'; }; }
  if (typeof StylePropertyMapReadOnly === 'undefined') { window.StylePropertyMapReadOnly = function() { this[Symbol.toStringTag] = 'StylePropertyMapReadOnly'; }; }
  if (typeof StyleSheet === 'undefined') { window.StyleSheet = function() { this[Symbol.toStringTag] = 'StyleSheet'; }; }
  if (typeof StyleSheetList === 'undefined') { window.StyleSheetList = function() { this[Symbol.toStringTag] = 'StyleSheetList'; }; }
  if (typeof SubmitEvent === 'undefined') { window.SubmitEvent = function() { this[Symbol.toStringTag] = 'SubmitEvent'; }; }
  if (typeof Subscriber === 'undefined') { window.Subscriber = function() { this[Symbol.toStringTag] = 'Subscriber'; }; }
  if (typeof Summarizer === 'undefined') { window.Summarizer = function() { this[Symbol.toStringTag] = 'Summarizer'; }; }
  if (typeof SuppressedError === 'undefined') { window.SuppressedError = function() { this[Symbol.toStringTag] = 'SuppressedError'; }; }
  if (typeof SyncManager === 'undefined') { window.SyncManager = function() { this[Symbol.toStringTag] = 'SyncManager'; }; }
  if (typeof TaskAttributionTiming === 'undefined') { window.TaskAttributionTiming = function() { this[Symbol.toStringTag] = 'TaskAttributionTiming'; }; }
  if (typeof TaskController === 'undefined') { window.TaskController = function() { this[Symbol.toStringTag] = 'TaskController'; }; }
  if (typeof TaskPriorityChangeEvent === 'undefined') { window.TaskPriorityChangeEvent = function() { this[Symbol.toStringTag] = 'TaskPriorityChangeEvent'; }; }
  if (typeof TaskSignal === 'undefined') { window.TaskSignal = function() { this[Symbol.toStringTag] = 'TaskSignal'; }; }
  if (typeof Text === 'undefined') { window.Text = function() { this[Symbol.toStringTag] = 'Text'; }; }
  if (typeof TextEvent === 'undefined') { window.TextEvent = function() { this[Symbol.toStringTag] = 'TextEvent'; }; }
  if (typeof TextFormat === 'undefined') { window.TextFormat = function() { this[Symbol.toStringTag] = 'TextFormat'; }; }
  if (typeof TextFormatUpdateEvent === 'undefined') { window.TextFormatUpdateEvent = function() { this[Symbol.toStringTag] = 'TextFormatUpdateEvent'; }; }
  if (typeof TextMetrics === 'undefined') { window.TextMetrics = function() { this[Symbol.toStringTag] = 'TextMetrics'; }; }
  if (typeof TextTrack === 'undefined') { window.TextTrack = function() { this[Symbol.toStringTag] = 'TextTrack'; }; }
  if (typeof TextTrackCue === 'undefined') { window.TextTrackCue = function() { this[Symbol.toStringTag] = 'TextTrackCue'; }; }
  if (typeof TextTrackCueList === 'undefined') { window.TextTrackCueList = function() { this[Symbol.toStringTag] = 'TextTrackCueList'; }; }
  if (typeof TextTrackList === 'undefined') { window.TextTrackList = function() { this[Symbol.toStringTag] = 'TextTrackList'; }; }
  if (typeof TextUpdateEvent === 'undefined') { window.TextUpdateEvent = function() { this[Symbol.toStringTag] = 'TextUpdateEvent'; }; }
  if (typeof TimeRanges === 'undefined') { window.TimeRanges = function() { this[Symbol.toStringTag] = 'TimeRanges'; }; }
  if (typeof TimelineTrigger === 'undefined') { window.TimelineTrigger = function() { this[Symbol.toStringTag] = 'TimelineTrigger'; }; }
  if (typeof TimelineTriggerRange === 'undefined') { window.TimelineTriggerRange = function() { this[Symbol.toStringTag] = 'TimelineTriggerRange'; }; }
  if (typeof TimelineTriggerRangeList === 'undefined') { window.TimelineTriggerRangeList = function() { this[Symbol.toStringTag] = 'TimelineTriggerRangeList'; }; }
  if (typeof ToggleEvent === 'undefined') { window.ToggleEvent = function() { this[Symbol.toStringTag] = 'ToggleEvent'; }; }
  if (typeof Touch === 'undefined') { window.Touch = function() { this[Symbol.toStringTag] = 'Touch'; }; }
  if (typeof TouchEvent === 'undefined') { window.TouchEvent = function() { this[Symbol.toStringTag] = 'TouchEvent'; }; }
  if (typeof TouchList === 'undefined') { window.TouchList = function() { this[Symbol.toStringTag] = 'TouchList'; }; }
  if (typeof TrackEvent === 'undefined') { window.TrackEvent = function() { this[Symbol.toStringTag] = 'TrackEvent'; }; }
  if (typeof TransformStreamDefaultController === 'undefined') { window.TransformStreamDefaultController = function() { this[Symbol.toStringTag] = 'TransformStreamDefaultController'; }; }
  if (typeof TransitionEvent === 'undefined') { window.TransitionEvent = function() { this[Symbol.toStringTag] = 'TransitionEvent'; }; }
  if (typeof Translator === 'undefined') { window.Translator = function() { this[Symbol.toStringTag] = 'Translator'; }; }
  if (typeof TreeWalker === 'undefined') { window.TreeWalker = function() { this[Symbol.toStringTag] = 'TreeWalker'; }; }
  if (typeof TrustedHTML === 'undefined') { window.TrustedHTML = function() { this[Symbol.toStringTag] = 'TrustedHTML'; }; }
  if (typeof TrustedScript === 'undefined') { window.TrustedScript = function() { this[Symbol.toStringTag] = 'TrustedScript'; }; }
  if (typeof TrustedScriptURL === 'undefined') { window.TrustedScriptURL = function() { this[Symbol.toStringTag] = 'TrustedScriptURL'; }; }
  if (typeof TrustedTypePolicy === 'undefined') { window.TrustedTypePolicy = function() { this[Symbol.toStringTag] = 'TrustedTypePolicy'; }; }
  if (typeof TrustedTypePolicyFactory === 'undefined') { window.TrustedTypePolicyFactory = function() { this[Symbol.toStringTag] = 'TrustedTypePolicyFactory'; }; }
  if (typeof UIEvent === 'undefined') { window.UIEvent = function() { this[Symbol.toStringTag] = 'UIEvent'; }; }
  if (typeof UserActivation === 'undefined') { window.UserActivation = function() { this[Symbol.toStringTag] = 'UserActivation'; }; }
  if (typeof VTTCue === 'undefined') { window.VTTCue = function() { this[Symbol.toStringTag] = 'VTTCue'; }; }
  if (typeof ValidityState === 'undefined') { window.ValidityState = function() { this[Symbol.toStringTag] = 'ValidityState'; }; }
  if (typeof VideoColorSpace === 'undefined') { window.VideoColorSpace = function() { this[Symbol.toStringTag] = 'VideoColorSpace'; }; }
  if (typeof VideoDecoder === 'undefined') { window.VideoDecoder = function() { this[Symbol.toStringTag] = 'VideoDecoder'; }; }
  if (typeof VideoEncoder === 'undefined') { window.VideoEncoder = function() { this[Symbol.toStringTag] = 'VideoEncoder'; }; }
  if (typeof VideoFrame === 'undefined') { window.VideoFrame = function() { this[Symbol.toStringTag] = 'VideoFrame'; }; }
  if (typeof VideoPlaybackQuality === 'undefined') { window.VideoPlaybackQuality = function() { this[Symbol.toStringTag] = 'VideoPlaybackQuality'; }; }
  if (typeof ViewTimeline === 'undefined') { window.ViewTimeline = function() { this[Symbol.toStringTag] = 'ViewTimeline'; }; }
  if (typeof ViewTransition === 'undefined') { window.ViewTransition = function() { this[Symbol.toStringTag] = 'ViewTransition'; }; }
  if (typeof ViewTransitionTypeSet === 'undefined') { window.ViewTransitionTypeSet = function() { this[Symbol.toStringTag] = 'ViewTransitionTypeSet'; }; }
  if (typeof Viewport === 'undefined') { window.Viewport = function() { this[Symbol.toStringTag] = 'Viewport'; }; }
  if (typeof VirtualKeyboard === 'undefined') { window.VirtualKeyboard = function() { this[Symbol.toStringTag] = 'VirtualKeyboard'; }; }
  if (typeof VirtualKeyboardGeometryChangeEvent === 'undefined') { window.VirtualKeyboardGeometryChangeEvent = function() { this[Symbol.toStringTag] = 'VirtualKeyboardGeometryChangeEvent'; }; }
  if (typeof VisibilityStateEntry === 'undefined') { window.VisibilityStateEntry = function() { this[Symbol.toStringTag] = 'VisibilityStateEntry'; }; }
  if (typeof VisualViewport === 'undefined') { window.VisualViewport = function() { this[Symbol.toStringTag] = 'VisualViewport'; }; }
  if (typeof WGSLLanguageFeatures === 'undefined') { window.WGSLLanguageFeatures = function() { this[Symbol.toStringTag] = 'WGSLLanguageFeatures'; }; }
  if (typeof WakeLock === 'undefined') { window.WakeLock = function() { this[Symbol.toStringTag] = 'WakeLock'; }; }
  if (typeof WakeLockSentinel === 'undefined') { window.WakeLockSentinel = function() { this[Symbol.toStringTag] = 'WakeLockSentinel'; }; }
  if (typeof WaveShaperNode === 'undefined') { window.WaveShaperNode = function() { this[Symbol.toStringTag] = 'WaveShaperNode'; }; }
  if (typeof WebGLActiveInfo === 'undefined') { window.WebGLActiveInfo = function() { this[Symbol.toStringTag] = 'WebGLActiveInfo'; }; }
  if (typeof WebGLBuffer === 'undefined') { window.WebGLBuffer = function() { this[Symbol.toStringTag] = 'WebGLBuffer'; }; }
  if (typeof WebGLContextEvent === 'undefined') { window.WebGLContextEvent = function() { this[Symbol.toStringTag] = 'WebGLContextEvent'; }; }
  if (typeof WebGLFramebuffer === 'undefined') { window.WebGLFramebuffer = function() { this[Symbol.toStringTag] = 'WebGLFramebuffer'; }; }
  if (typeof WebGLObject === 'undefined') { window.WebGLObject = function() { this[Symbol.toStringTag] = 'WebGLObject'; }; }
  if (typeof WebGLProgram === 'undefined') { window.WebGLProgram = function() { this[Symbol.toStringTag] = 'WebGLProgram'; }; }
  if (typeof WebGLQuery === 'undefined') { window.WebGLQuery = function() { this[Symbol.toStringTag] = 'WebGLQuery'; }; }
  if (typeof WebGLRenderbuffer === 'undefined') { window.WebGLRenderbuffer = function() { this[Symbol.toStringTag] = 'WebGLRenderbuffer'; }; }
  if (typeof WebGLSampler === 'undefined') { window.WebGLSampler = function() { this[Symbol.toStringTag] = 'WebGLSampler'; }; }
  if (typeof WebGLShader === 'undefined') { window.WebGLShader = function() { this[Symbol.toStringTag] = 'WebGLShader'; }; }
  if (typeof WebGLShaderPrecisionFormat === 'undefined') { window.WebGLShaderPrecisionFormat = function() { this[Symbol.toStringTag] = 'WebGLShaderPrecisionFormat'; }; }
  if (typeof WebGLSync === 'undefined') { window.WebGLSync = function() { this[Symbol.toStringTag] = 'WebGLSync'; }; }
  if (typeof WebGLTexture === 'undefined') { window.WebGLTexture = function() { this[Symbol.toStringTag] = 'WebGLTexture'; }; }
  if (typeof WebGLTransformFeedback === 'undefined') { window.WebGLTransformFeedback = function() { this[Symbol.toStringTag] = 'WebGLTransformFeedback'; }; }
  if (typeof WebGLUniformLocation === 'undefined') { window.WebGLUniformLocation = function() { this[Symbol.toStringTag] = 'WebGLUniformLocation'; }; }
  if (typeof WebGLVertexArrayObject === 'undefined') { window.WebGLVertexArrayObject = function() { this[Symbol.toStringTag] = 'WebGLVertexArrayObject'; }; }
  if (typeof WebKitCSSMatrix === 'undefined') { window.WebKitCSSMatrix = function() { this[Symbol.toStringTag] = 'WebKitCSSMatrix'; }; }
  if (typeof WebKitMutationObserver === 'undefined') { window.WebKitMutationObserver = function() { this[Symbol.toStringTag] = 'WebKitMutationObserver'; }; }
  if (typeof WebSocketError === 'undefined') { window.WebSocketError = function() { this[Symbol.toStringTag] = 'WebSocketError'; }; }
  if (typeof WebSocketStream === 'undefined') { window.WebSocketStream = function() { this[Symbol.toStringTag] = 'WebSocketStream'; }; }
  if (typeof WebTransport === 'undefined') { window.WebTransport = function() { this[Symbol.toStringTag] = 'WebTransport'; }; }
  if (typeof WebTransportBidirectionalStream === 'undefined') { window.WebTransportBidirectionalStream = function() { this[Symbol.toStringTag] = 'WebTransportBidirectionalStream'; }; }
  if (typeof WebTransportDatagramDuplexStream === 'undefined') { window.WebTransportDatagramDuplexStream = function() { this[Symbol.toStringTag] = 'WebTransportDatagramDuplexStream'; }; }
  if (typeof WebTransportError === 'undefined') { window.WebTransportError = function() { this[Symbol.toStringTag] = 'WebTransportError'; }; }
  if (typeof WheelEvent === 'undefined') { window.WheelEvent = function() { this[Symbol.toStringTag] = 'WheelEvent'; }; }
  if (typeof Window === 'undefined') { window.Window = function() { this[Symbol.toStringTag] = 'Window'; }; }
  if (typeof WindowControlsOverlay === 'undefined') { window.WindowControlsOverlay = function() { this[Symbol.toStringTag] = 'WindowControlsOverlay'; }; }
  if (typeof WindowControlsOverlayGeometryChangeEvent === 'undefined') { window.WindowControlsOverlayGeometryChangeEvent = function() { this[Symbol.toStringTag] = 'WindowControlsOverlayGeometryChangeEvent'; }; }
  if (typeof Worklet === 'undefined') { window.Worklet = function() { this[Symbol.toStringTag] = 'Worklet'; }; }
  if (typeof WritableStreamDefaultController === 'undefined') { window.WritableStreamDefaultController = function() { this[Symbol.toStringTag] = 'WritableStreamDefaultController'; }; }
  if (typeof WritableStreamDefaultWriter === 'undefined') { window.WritableStreamDefaultWriter = function() { this[Symbol.toStringTag] = 'WritableStreamDefaultWriter'; }; }
  if (typeof XMLDocument === 'undefined') { window.XMLDocument = function() { this[Symbol.toStringTag] = 'XMLDocument'; }; }
  if (typeof XMLHttpRequestEventTarget === 'undefined') { window.XMLHttpRequestEventTarget = function() { this[Symbol.toStringTag] = 'XMLHttpRequestEventTarget'; }; }
  if (typeof XMLHttpRequestUpload === 'undefined') { window.XMLHttpRequestUpload = function() { this[Symbol.toStringTag] = 'XMLHttpRequestUpload'; }; }
  if (typeof XMLSerializer === 'undefined') { window.XMLSerializer = function() { this[Symbol.toStringTag] = 'XMLSerializer'; }; }
  if (typeof XPathEvaluator === 'undefined') { window.XPathEvaluator = function() { this[Symbol.toStringTag] = 'XPathEvaluator'; }; }
  if (typeof XPathExpression === 'undefined') { window.XPathExpression = function() { this[Symbol.toStringTag] = 'XPathExpression'; }; }
  if (typeof XPathResult === 'undefined') { window.XPathResult = function() { this[Symbol.toStringTag] = 'XPathResult'; }; }
  if (typeof XRAnchor === 'undefined') { window.XRAnchor = function() { this[Symbol.toStringTag] = 'XRAnchor'; }; }
  if (typeof XRAnchorSet === 'undefined') { window.XRAnchorSet = function() { this[Symbol.toStringTag] = 'XRAnchorSet'; }; }
  if (typeof XRBoundedReferenceSpace === 'undefined') { window.XRBoundedReferenceSpace = function() { this[Symbol.toStringTag] = 'XRBoundedReferenceSpace'; }; }
  if (typeof XRCPUDepthInformation === 'undefined') { window.XRCPUDepthInformation = function() { this[Symbol.toStringTag] = 'XRCPUDepthInformation'; }; }
  if (typeof XRCamera === 'undefined') { window.XRCamera = function() { this[Symbol.toStringTag] = 'XRCamera'; }; }
  if (typeof XRDOMOverlayState === 'undefined') { window.XRDOMOverlayState = function() { this[Symbol.toStringTag] = 'XRDOMOverlayState'; }; }
  if (typeof XRDepthInformation === 'undefined') { window.XRDepthInformation = function() { this[Symbol.toStringTag] = 'XRDepthInformation'; }; }
  if (typeof XRFrame === 'undefined') { window.XRFrame = function() { this[Symbol.toStringTag] = 'XRFrame'; }; }
  if (typeof XRHand === 'undefined') { window.XRHand = function() { this[Symbol.toStringTag] = 'XRHand'; }; }
  if (typeof XRHitTestResult === 'undefined') { window.XRHitTestResult = function() { this[Symbol.toStringTag] = 'XRHitTestResult'; }; }
  if (typeof XRHitTestSource === 'undefined') { window.XRHitTestSource = function() { this[Symbol.toStringTag] = 'XRHitTestSource'; }; }
  if (typeof XRInputSource === 'undefined') { window.XRInputSource = function() { this[Symbol.toStringTag] = 'XRInputSource'; }; }
  if (typeof XRInputSourceArray === 'undefined') { window.XRInputSourceArray = function() { this[Symbol.toStringTag] = 'XRInputSourceArray'; }; }
  if (typeof XRInputSourceEvent === 'undefined') { window.XRInputSourceEvent = function() { this[Symbol.toStringTag] = 'XRInputSourceEvent'; }; }
  if (typeof XRInputSourcesChangeEvent === 'undefined') { window.XRInputSourcesChangeEvent = function() { this[Symbol.toStringTag] = 'XRInputSourcesChangeEvent'; }; }
  if (typeof XRJointPose === 'undefined') { window.XRJointPose = function() { this[Symbol.toStringTag] = 'XRJointPose'; }; }
  if (typeof XRJointSpace === 'undefined') { window.XRJointSpace = function() { this[Symbol.toStringTag] = 'XRJointSpace'; }; }
  if (typeof XRLayer === 'undefined') { window.XRLayer = function() { this[Symbol.toStringTag] = 'XRLayer'; }; }
  if (typeof XRLightEstimate === 'undefined') { window.XRLightEstimate = function() { this[Symbol.toStringTag] = 'XRLightEstimate'; }; }
  if (typeof XRLightProbe === 'undefined') { window.XRLightProbe = function() { this[Symbol.toStringTag] = 'XRLightProbe'; }; }
  if (typeof XRPose === 'undefined') { window.XRPose = function() { this[Symbol.toStringTag] = 'XRPose'; }; }
  if (typeof XRRay === 'undefined') { window.XRRay = function() { this[Symbol.toStringTag] = 'XRRay'; }; }
  if (typeof XRReferenceSpace === 'undefined') { window.XRReferenceSpace = function() { this[Symbol.toStringTag] = 'XRReferenceSpace'; }; }
  if (typeof XRReferenceSpaceEvent === 'undefined') { window.XRReferenceSpaceEvent = function() { this[Symbol.toStringTag] = 'XRReferenceSpaceEvent'; }; }
  if (typeof XRRenderState === 'undefined') { window.XRRenderState = function() { this[Symbol.toStringTag] = 'XRRenderState'; }; }
  if (typeof XRRigidTransform === 'undefined') { window.XRRigidTransform = function() { this[Symbol.toStringTag] = 'XRRigidTransform'; }; }
  if (typeof XRSession === 'undefined') { window.XRSession = function() { this[Symbol.toStringTag] = 'XRSession'; }; }
  if (typeof XRSessionEvent === 'undefined') { window.XRSessionEvent = function() { this[Symbol.toStringTag] = 'XRSessionEvent'; }; }
  if (typeof XRSpace === 'undefined') { window.XRSpace = function() { this[Symbol.toStringTag] = 'XRSpace'; }; }
  if (typeof XRSystem === 'undefined') { window.XRSystem = function() { this[Symbol.toStringTag] = 'XRSystem'; }; }
  if (typeof XRTransientInputHitTestResult === 'undefined') { window.XRTransientInputHitTestResult = function() { this[Symbol.toStringTag] = 'XRTransientInputHitTestResult'; }; }
  if (typeof XRTransientInputHitTestSource === 'undefined') { window.XRTransientInputHitTestSource = function() { this[Symbol.toStringTag] = 'XRTransientInputHitTestSource'; }; }
  if (typeof XRView === 'undefined') { window.XRView = function() { this[Symbol.toStringTag] = 'XRView'; }; }
  if (typeof XRViewerPose === 'undefined') { window.XRViewerPose = function() { this[Symbol.toStringTag] = 'XRViewerPose'; }; }
  if (typeof XRViewport === 'undefined') { window.XRViewport = function() { this[Symbol.toStringTag] = 'XRViewport'; }; }
  if (typeof XRVisibilityMaskChangeEvent === 'undefined') { window.XRVisibilityMaskChangeEvent = function() { this[Symbol.toStringTag] = 'XRVisibilityMaskChangeEvent'; }; }
  if (typeof XRWebGLBinding === 'undefined') { window.XRWebGLBinding = function() { this[Symbol.toStringTag] = 'XRWebGLBinding'; }; }
  if (typeof XRWebGLDepthInformation === 'undefined') { window.XRWebGLDepthInformation = function() { this[Symbol.toStringTag] = 'XRWebGLDepthInformation'; }; }
  if (typeof XRWebGLLayer === 'undefined') { window.XRWebGLLayer = function() { this[Symbol.toStringTag] = 'XRWebGLLayer'; }; }
  if (typeof XSLTProcessor === 'undefined') { window.XSLTProcessor = function() { this[Symbol.toStringTag] = 'XSLTProcessor'; }; }

})();