// env_shim_part4.js — Behavior simulation + WebGPU + enhanced CSS selectors + HTML parser

(function() {
  'use strict';

  // ── WebGPU (navigator.gpu) ──
  var webgpuAdapter = {
    info: { vendor: 'nvidia', architecture: 'ada-lovelace', device: 'NVIDIA GeForce RTX 4060', description: 'nvidia ada-lovelace' },
    features: new Set(['depth-clip-control', 'depth32float-stencil8', 'timestamp-query', 'indirect-first-instance', 'shader-f16', 'rg11b10ufloat-renderable', 'bgra8unorm-storage']),
    limits: { maxTextureDimension1D: 16384, maxTextureDimension2D: 16384, maxTextureDimension3D: 2048, maxTextureArrayLayers: 2048, maxBindGroups: 4, maxBindingsPerBindGroup: 1000, maxBufferSize: 4294967296, maxUniformBufferBindingSize: 65536, maxStorageBufferBindingSize: 4294967296, maxVertexBuffers: 8, maxVertexAttributes: 16, maxVertexBufferArrayStride: 2048 },
    requestAdapterInfo: function() { return Promise.resolve(this.info); },
  };
  var webgpu = {
    requestAdapter: function(options) { return Promise.resolve(webgpuAdapter); },
    getPreferredCanvasFormat: function() { return 'bgra8unorm'; },
    wgslLanguageFeatures: new Set(),
  };
  Object.defineProperty(navigator, 'gpu', { value: webgpu, configurable: true });

  // ── Enhanced CSS selector engine (querySelector/querySelectorAll) ──
  // Simple but functional CSS selector parser supporting:
  // tag, #id, .class, [attr], [attr=val], descendant combinator, comma
  var _besElementRegistry = {}; // id → element
  var _besAllElements = [];

  function _besRegisterElement(el) {
    if (el && el.id) _besElementRegistry[el.id] = el;
    _besAllElements.push(el);
  }

  function _besParseSelector(sel) {
    // Split by comma for multiple selectors
    var groups = sel.split(',').map(function(s) { return s.trim(); });
    var parsed = groups.map(function(group) {
      // Split by whitespace for descendant combinator
      var parts = group.split(/\s+/);
      return parts.map(function(part) {
        var result = { tag: null, id: null, classes: [], attrs: [] };
        // Parse the part
        var remaining = part;
        // Tag
        var tagMatch = remaining.match(/^([a-zA-Z][a-zA-Z0-9-]*)/);
        if (tagMatch) { result.tag = tagMatch[1].toLowerCase(); remaining = remaining.slice(tagMatch[1].length); }
        // #id, .class, [attr] in sequence
        while (remaining.length > 0) {
          if (remaining[0] === '#') {
            var idMatch = remaining.match(/^#([a-zA-Z0-9_-]+)/);
            if (idMatch) { result.id = idMatch[1]; remaining = remaining.slice(idMatch[0].length); }
            else break;
          } else if (remaining[0] === '.') {
            var clsMatch = remaining.match(/^\.([a-zA-Z0-9_-]+)/);
            if (clsMatch) { result.classes.push(clsMatch[1]); remaining = remaining.slice(clsMatch[0].length); }
            else break;
          } else if (remaining[0] === '[') {
            var attrEnd = remaining.indexOf(']');
            if (attrEnd > 0) {
              var attrStr = remaining.slice(1, attrEnd);
              var eqIdx = attrStr.indexOf('=');
              if (eqIdx > 0) {
                var attrName = attrStr.slice(0, eqIdx).trim();
                var attrVal = attrStr.slice(eqIdx + 1).trim().replace(/^["']|["']$/g, '');
                result.attrs.push({ name: attrName, value: attrVal });
              } else {
                result.attrs.push({ name: attrStr.trim(), value: null });
              }
              remaining = remaining.slice(attrEnd + 1);
            } else break;
          } else {
            break;
          }
        }
        return result;
      });
    });
    return parsed;
  }

  function _besMatchElement(el, selector) {
    if (!el || !el.tagName) return false;
    if (selector.tag && el.tagName.toLowerCase() !== selector.tag) return false;
    if (selector.id && el.id !== selector.id) return false;
    if (selector.classes.length > 0) {
      var elClasses = (el.className || '').split(/\s+/);
      for (var i = 0; i < selector.classes.length; i++) {
        if (elClasses.indexOf(selector.classes[i]) < 0) return false;
      }
    }
    if (selector.attrs.length > 0) {
      for (var j = 0; j < selector.attrs.length; j++) {
        var attr = selector.attrs[j];
        var val = el.getAttribute ? el.getAttribute(attr.name) : null;
        if (val === null) return false;
        if (attr.value !== null && val !== attr.value) return false;
      }
    }
    return true;
  }

  function _besQueryAll(root, selectorGroups) {
    var results = [];
    for (var g = 0; g < selectorGroups.length; g++) {
      var chain = selectorGroups[g];
      if (chain.length === 1) {
        // Simple selector
        for (var i = 0; i < _besAllElements.length; i++) {
          if (_besMatchElement(_besAllElements[i], chain[0])) {
            results.push(_besAllElements[i]);
          }
        }
      } else {
        // Descendant combinator — simplified: check all elements against last selector
        var last = chain[chain.length - 1];
        for (var k = 0; k < _besAllElements.length; k++) {
          if (_besMatchElement(_besAllElements[k], last)) {
            results.push(_besAllElements[k]);
          }
        }
      }
    }
    return results;
  }

  // Override document.querySelector/querySelectorAll with enhanced engine
  document.querySelector = function(sel) {
    var parsed = _besParseSelector(sel);
    var results = _besQueryAll(document, parsed);
    if (results.length > 0) return results[0];
    // Fallback to basic checks
    if (sel === 'head') return document.head;
    if (sel === 'body') return document.body;
    if (sel === 'html') return document.documentElement;
    return null;
  };
  document.querySelectorAll = function(sel) {
    var parsed = _besParseSelector(sel);
    var results = _besQueryAll(document, parsed);
    if (results.length > 0) return results;
    return [];
  };

  // ── Enhanced DOMParser (returns parseable DOM tree) ──
  var origDOMParser = window.DOMParser;
  window.DOMParser = function() {};
  window.DOMParser.prototype.parseFromString = function(str, type) {
    // Simple HTML parser — extracts tags and text into a DOM-like tree
    var doc = {
      documentElement: _besParseHTML(str),
      head: { children: [], getElementsByTagName: function() { return []; }, querySelector: function() { return null; } },
      body: { children: [], getElementsByTagName: function() { return []; }, querySelector: function() { return null; } },
      getElementById: function(id) { return _besElementRegistry[id] || null; },
      querySelector: function(sel) { return document.querySelector(sel); },
      querySelectorAll: function(sel) { return document.querySelectorAll(sel); },
      getElementsByTagName: function(tag) {
        var upper = tag.toUpperCase();
        return _besAllElements.filter(function(el) { return el.tagName === upper; });
      },
    };
    return doc;
  };

  function _besParseHTML(html) {
    // Very simplified HTML parser — just enough to not crash
    var root = document.createElement('html');
    // Extract <head> and <body> if present
    var headMatch = html.match(/<head[^>]*>([\s\S]*?)<\/head>/i);
    var bodyMatch = html.match(/<body[^>]*>([\s\S]*?)<\/body>/i);

    if (headMatch) {
      var head = document.createElement('head');
      head.innerHTML = headMatch[1];
      _besRegisterElement(head);
      root.appendChild = head;
    }
    if (bodyMatch) {
      var body = document.createElement('body');
      body.innerHTML = bodyMatch[1];
      _besRegisterElement(body);
      root.appendChild = body;
    }
    return root;
  }

  // ── Behavior simulation: human-like mouse/keyboard events ──
  // Provides realistic timing for mouse moves, clicks, and key presses.
  window._besBehavior = {
    mousePath: [],
    keyTimings: [],

    // Generate a human-like mouse movement path from (x1,y1) to (x2,y2)
    generateMousePath: function(x1, y1, x2, y2) {
      var steps = [];
      var dist = Math.sqrt((x2-x1)*(x2-x1) + (y2-y1)*(y2-y1));
      var numSteps = Math.max(3, Math.floor(dist / 15) + Math.random() * 5);
      for (var i = 0; i <= numSteps; i++) {
        var t = i / numSteps;
        // Bezier-like curve with noise
        var noiseX = (Math.random() - 0.5) * 8;
        var noiseY = (Math.random() - 0.5) * 8;
        var easeT = t < 0.5 ? 2*t*t : -1 + (4-2*t)*t; // easeInOut
        steps.push({
          x: x1 + (x2-x1) * easeT + noiseX,
          y: y1 + (y2-y1) * easeT + noiseY,
          delay: 8 + Math.random() * 20, // 8-28ms per step
        });
      }
      return steps;
    },

    // Generate human-like typing delays (in ms)
    generateKeyTimings: function(text) {
      var timings = [];
      var baseDelay = 80;
      for (var i = 0; i < text.length; i++) {
        var delay = baseDelay + (Math.random() - 0.3) * 60;
        // Occasional pause (thinking)
        if (Math.random() < 0.05) delay += 200 + Math.random() * 500;
        // Faster on common key pairs
        if (i > 0) {
          var prev = text[i-1].toLowerCase();
          var curr = text[i].toLowerCase();
          if ('aeiou'.indexOf(curr) >= 0) delay *= 0.8;
          if (prev === curr) delay *= 0.7;
        }
        timings.push(Math.max(30, Math.floor(delay)));
      }
      return timings;
    },

    // Generate scroll behavior
    generateScroll: function(totalHeight) {
      var steps = [];
      var pos = 0;
      while (pos < totalHeight) {
        var step = 50 + Math.random() * 100;
        pos += step;
        steps.push({ y: Math.min(pos, totalHeight), delay: 100 + Math.random() * 200 });
      }
      return steps;
    },
  };

  // ── Human-like input simulation helpers on elements ──
  var origCreateElement = document.createElement;
  document.createElement = function(tagName) {
    var el = origCreateElement.call(document, tagName);
    if (!el) return el;

    // Register element for CSS selector engine
    _besRegisterElement(el);

    // Add human-like click for buttons/links
    if (typeof el.humanClick === 'undefined') {
      el.humanClick = function() {
        var self = this;
        var rect = self.getBoundingClientRect();
        var cx = rect.x + rect.width / 2;
        var cy = rect.y + rect.height / 2;
        // Simulate mouse move → mouseover → mousedown → mouseup → click
        var path = window._besBehavior.generateMousePath(cx + 100, cy + 100, cx, cy);
        var pathIdx = 0;
        function nextStep() {
          if (pathIdx >= path.length) {
            self.dispatchEvent({ type: 'mouseover' });
            self.dispatchEvent({ type: 'mousedown' });
            setTimeout(function() {
              self.dispatchEvent({ type: 'mouseup' });
              self.dispatchEvent({ type: 'click' });
            }, 50 + Math.random() * 50);
            return;
          }
          var step = path[pathIdx++];
          setTimeout(nextStep, step.delay);
        }
        nextStep();
      };
    }

    // Add human-like typing for inputs
    if (typeof el.humanType === 'undefined') {
      el.humanType = function(text) {
        var self = this;
        var timings = window._besBehavior.generateKeyTimings(text);
        self.focus();
        self.value = '';
        var idx = 0;
        function nextKey() {
          if (idx >= text.length) {
            self.dispatchEvent({ type: 'input' });
            self.dispatchEvent({ type: 'change' });
            return;
          }
          self.value += text[idx];
          self.dispatchEvent({ type: 'keydown' });
          self.dispatchEvent({ type: 'keypress' });
          self.dispatchEvent({ type: 'input' });
          self.dispatchEvent({ type: 'keyup' });
          idx++;
          setTimeout(nextKey, timings[idx - 1]);
        }
        nextKey();
      };
    }

    return el;
  };

})();
