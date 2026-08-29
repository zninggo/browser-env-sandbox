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

  // ── Enhanced DOMParser (returns real DOM tree) ──
  // Tree-scoped search helpers (avoid global _besAllElements pollution).
  function _besFindByTagInTree(root, upperTag) {
    var results = [];
    function walk(node) {
      if (!node) return;
      if (node.tagName === upperTag) results.push(node);
      var children = node.children || [];
      for (var i = 0; i < children.length; i++) walk(children[i]);
      // Also search template content fragments
      if (node.content && node.content.children) {
        for (var j = 0; j < node.content.children.length; j++) walk(node.content.children[j]);
      }
    }
    walk(root);
    return results;
  }
  function _besFindByIdInTree(root, id) {
    function walk(node) {
      if (!node) return null;
      if (node.id === id) return node;
      var children = node.children || [];
      for (var i = 0; i < children.length; i++) {
        var found = walk(children[i]);
        if (found) return found;
      }
      if (node.content && node.content.children) {
        for (var j = 0; j < node.content.children.length; j++) {
          var found2 = walk(node.content.children[j]);
          if (found2) return found2;
        }
      }
      return null;
    }
    return walk(root);
  }
  // A recursive-descent HTML tokenizer that builds a tree of real DOM
  // elements (created via document.createElement) with parent/child links,
  // attributes, and text nodes — queryable through the existing CSS selector
  // engine and getElementsByTagName.
  var origDOMParser = window.DOMParser;
  window.DOMParser = function() {};
  window.DOMParser.prototype.parseFromString = function(str, type) {
    var root = _besParseHTML(str || '');
    var headEl = null, bodyEl = null;
    for (var i = 0; i < root.children.length; i++) {
      var c = root.children[i];
      if (c.tagName === 'HEAD') headEl = c;
      else if (c.tagName === 'BODY') bodyEl = c;
    }
    var doc = {
      documentElement: root,
      head: headEl || { children: [], getElementsByTagName: function() { return []; }, querySelector: function() { return null; } },
      body: bodyEl || { children: [], getElementsByTagName: function() { return []; }, querySelector: function() { return null; } },
      getElementById: function(id) {
        // Search within this parsed tree only
        return _besFindByIdInTree(root, id);
      },
      querySelector: function(sel) { return document.querySelector(sel); },
      querySelectorAll: function(sel) { return document.querySelectorAll(sel); },
      getElementsByTagName: function(tag) {
        var upper = tag.toUpperCase();
        return _besFindByTagInTree(root, upper);
      },
    };
    return doc;
  };

  // _besParseHTML: tokenizes HTML into a tree of DOM elements.
  // Handles: tags (open/close/self-closing), attributes, text nodes, void
  // elements, auto-closing of <p>/<li>/<td>/<tr>/<option> (HTML5 quirk),
  // <template> content, and <svg>/[itex] foreign content passthrough.
  function _besParseHTML(html) {
    var root = document.createElement('html');
    var stack = [root];
    var voidTags = { BR:1, IMG:1, INPUT:1, META:1, LINK:1, HR:1, AREA:1, BASE:1, COL:1, EMBED:1, PARAM:1, SOURCE:1, TRACK:1, WBR:1 };
    // Tags that auto-close when a sibling of the same/related type opens.
    // HTML5 spec: <p> closes on block elements; <li> closes on next <li>;
    // <td>/<th> close on next <td>/<th>/<tr>; <tr> closes on next <tr>.
    var autoCloseRules = {
      'P': ['P','DIV','UL','OL','TABLE','H1','H2','H3','H4','H5','H6','BLOCKQUOTE','PRE','SECTION','ARTICLE','HEADER','FOOTER','NAV','ASIDE','FIGURE'],
      'LI': ['LI'],
      'TD': ['TD','TH','TR'],
      'TH': ['TD','TH','TR'],
      'TR': ['TR'],
      'OPTION': ['OPTION','OPTGROUP'],
      'OPTGROUP': ['OPTGROUP'],
      'DD': ['DD','DT'],
      'DT': ['DD','DT'],
    };
    // Foreign content namespaces — parsed as opaque subtrees (content still
    // tokenized but namespace flagged so querySelector knows context).
    var foreignTags = { SVG:1, MATH:1 };
    var i = 0;
    while (i < html.length) {
      if (html[i] === '<') {
        // Comment or doctype?
        if (html.substr(i, 4) === '<!--') {
          var endC = html.indexOf('-->', i + 4);
          i = endC >= 0 ? endC + 4 : html.length;
          continue;
        }
        if (html.substr(i, 2) === '<!') {
          var endD = html.indexOf('>', i);
          i = endD >= 0 ? endD + 1 : html.length;
          continue;
        }
        // Closing tag?
        if (html[i+1] === '/') {
          var endClose = html.indexOf('>', i);
          if (endClose < 0) break;
          var closeTag = html.substring(i + 2, endClose).trim().toUpperCase();
          // Pop stack until we find the matching open tag
          for (var k = stack.length - 1; k >= 1; k--) {
            if (stack[k].tagName === closeTag) {
              stack.length = k;
              break;
            }
          }
          i = endClose + 1;
          continue;
        }
        // Opening tag — parse tag name + attributes
        var endTag = html.indexOf('>', i);
        if (endTag < 0) break;
        var tagContent = html.substring(i + 1, endTag);
        var selfClosing = tagContent.charAt(tagContent.length - 1) === '/';
        if (selfClosing) tagContent = tagContent.slice(0, -1).trim();
        // Extract tag name
        var spIdx = tagContent.search(/\s/);
        var tagName = (spIdx >= 0 ? tagContent.substring(0, spIdx) : tagContent).toUpperCase();
        if (!tagName) { i = endTag + 1; continue; }

        // Auto-close logic: check if current open tag should close for this new tag
        var currentTop = stack[stack.length - 1];
        if (currentTop && currentTop.tagName && autoCloseRules[currentTop.tagName]) {
          var closers = autoCloseRules[currentTop.tagName];
          if (closers.indexOf(tagName) >= 0) {
            stack.pop(); // auto-close the current element
          }
        }

        // Create element
        var el = document.createElement(tagName.toLowerCase());
        el.tagName = tagName;
        el.children = [];
        el.childNodes = [];
        // Flag foreign content
        if (foreignTags[tagName]) {
          el._besForeignNS = tagName.toLowerCase();
        } else if (currentTop && currentTop._besForeignNS) {
          el._besForeignNS = currentTop._besForeignNS;
        }
        // Parse attributes: name="value" | name='value' | name
        var attrStr = spIdx >= 0 ? tagContent.substring(spIdx + 1).trim() : '';
        var attrRe = /([a-zA-Z_:][a-zA-Z0-9_:.\-]*)\s*(?:=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+)))?/g;
        var am;
        while ((am = attrRe.exec(attrStr)) !== null) {
          var attrName = am[1];
          var attrVal = am[2] !== undefined ? am[2] : (am[3] !== undefined ? am[3] : (am[4] !== undefined ? am[4] : ''));
          try { el.setAttribute(attrName, attrVal); } catch(e) {}
          if (attrName.toLowerCase() === 'id') {
            try { el.id = attrVal; } catch(e) {}
            _besRegisterElement(el);
          } else if (attrName.toLowerCase() === 'class') {
            try { el.className = attrVal; } catch(e) {}
          }
        }
        _besRegisterElement(el);
        // <template>: store content as a documentFragment-like object
        if (tagName === 'TEMPLATE' && !selfClosing) {
          el.content = { children: [], childNodes: [], nodeType: 11 };
        }
        // Append to current parent. children = element nodes only;
        // childNodes = all nodes (elements + text).
        var parent = stack[stack.length - 1];
        // For <template> elements, children go into the content fragment.
        var appendTarget = parent;
        if (parent.tagName === 'TEMPLATE' && parent.content) {
          appendTarget = parent.content;
        }
        if (!appendTarget.children) appendTarget.children = [];
        if (!appendTarget.childNodes) appendTarget.childNodes = [];
        appendTarget.children.push(el);
        appendTarget.childNodes.push(el);
        el.parentNode = parent;
        el.parentElement = parent;
        // Push to stack unless void or self-closing
        if (!selfClosing && !voidTags[tagName]) {
          stack.push(el);
        }
        i = endTag + 1;
      } else {
        // Text node
        var endText = html.indexOf('<', i);
        if (endText < 0) endText = html.length;
        var text = html.substring(i, endText);
        if (text.trim()) {
          var parent2 = stack[stack.length - 1];
          var textNode = { nodeType: 3, textContent: text, nodeValue: text, parentNode: parent2 };
          // For <template>, append to content fragment instead of element itself
          if (parent2.tagName === 'TEMPLATE' && parent2.content) {
            parent2.content.children.push(textNode);
            parent2.content.childNodes = parent2.content.childNodes || [];
            parent2.content.childNodes.push(textNode);
          } else {
            // Text nodes go into childNodes only, NOT children (which is
            // element-only per DOM spec — matches browser querySelector behavior).
            if (!parent2.childNodes) parent2.childNodes = [];
            parent2.childNodes.push(textNode);
          }
        }
        i = endText;
      }
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

  // ── CAPTCHA solver bridge ──
  // Exposes window.bes.solveCaptcha as a JS-friendly wrapper over the Go
  // __besSolveCaptcha callback. Returns a Solution object synchronously;
  // for async use, wrap in Promise.
  if (typeof __besSolveCaptcha !== 'undefined') {
    if (typeof window.bes !== 'object') {
      window.bes = {};
    }
    window.bes.solveCaptcha = function(type, siteKey, pageURL, options) {
      var opts = options || {};
      var resultJSON = __besSolveCaptcha(type, siteKey, pageURL, JSON.stringify(opts));
      try { return JSON.parse(resultJSON); } catch(e) { return {solved: false, reason: String(e)}; }
    };
    // Convenience: async wrapper returning a Promise
    window.bes.solveCaptchaAsync = function(type, siteKey, pageURL, options) {
      return new Promise(function(resolve) {
        resolve(window.bes.solveCaptcha(type, siteKey, pageURL, options));
      });
    };
  }

})();
