package sandbox

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tommie/v8go"
)

// workerMessage is the wire format for worker ↔ parent envelopes. Payload is
// a JSON string produced JS-side (v8go callbacks exchange strings only —
// constraint #14).
type workerMessage struct {
	Type    string          `json:"type"` // "message", "error", "close"
	Payload json.RawMessage `json:"payload,omitempty"`
}

// workerRegistry tracks live workers per session so Dispose can tear them all
// down.
type workerRegistry struct {
	mu      sync.Mutex
	nextID  int32
	workers map[int32]*Worker
}

func newWorkerRegistry() *workerRegistry {
	return &workerRegistry{workers: make(map[int32]*Worker)}
}

// register assigns a unique worker id and tracks the worker.
func (r *workerRegistry) register(w *Worker) int32 {
	id := atomic.AddInt32(&r.nextID, 1)
	r.mu.Lock()
	r.workers[id] = w
	r.mu.Unlock()
	w.id = id
	return id
}

// get returns the worker with the given id (or nil).
func (r *workerRegistry) get(id int32) *Worker {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.workers[id]
}

// remove deletes and terminates the worker.
func (r *workerRegistry) remove(id int32) {
	r.mu.Lock()
	w, ok := r.workers[id]
	if ok {
		delete(r.workers, id)
	}
	r.mu.Unlock()
	if ok {
		w.terminate()
	}
}

// disposeAll terminates all workers (session teardown).
func (r *workerRegistry) disposeAll() {
	r.mu.Lock()
	workers := make([]*Worker, 0, len(r.workers))
	for _, w := range r.workers {
		workers = append(workers, w)
	}
	r.workers = make(map[int32]*Worker)
	r.mu.Unlock()
	for _, w := range workers {
		w.terminate()
	}
}

// Worker is a running Web Worker: a dedicated V8 Isolate executing the worker
// script, bridged to the parent sandbox via JSON envelopes.
//
// Threading model (V8 isolates are not thread-safe — constraint #11):
//   - The worker's Isolate is touched only from its own goroutine: script
//     run, message dispatch, and timer drains all happen there. The loop
//     blocks on the mailbox when idle, giving the worker an independent
//     event loop.
//   - The parent Isolate is only touched from the parent thread: worker→
//     parent replies are queued into the session's TimerManager callback
//     queue, which EvalAwait drains on the parent isolate thread.
type Worker struct {
	id         int32
	iso        *v8go.Isolate
	ctx        *v8go.Context
	timers     *TimerManager
	console    ConsoleSink
	userAgent  string
	inbound    chan string   // parent → worker mailbox (JSON envelopes)
	outbound   chan string   // worker → parent queue (JSON envelopes)
	stop       chan struct{} // closed by terminate()
	loopDone   chan struct{} // closed by workerLoop on exit
	terminated chan struct{} // closed by terminate() after full teardown
	once       sync.Once
}

// StartWorker creates and starts a Web Worker from JS source code.
// parentTimers is the parent session's timer manager (used by the outbound
// pump to deliver replies on the parent isolate thread). userAgent is the
// session fingerprint's UA so worker-side navigator.userAgent matches the
// parent.
func StartWorker(source string, parentTimers *TimerManager, console ConsoleSink, userAgent string) (*Worker, error) {
	iso := newIsolateWithLimits()
	ctx := v8go.NewContext(iso)
	w := &Worker{
		iso:        iso,
		ctx:        ctx,
		timers:     NewTimerManager(),
		console:    console,
		userAgent:  userAgent,
		inbound:    make(chan string, 64),
		outbound:   make(chan string, 64),
		stop:       make(chan struct{}),
		loopDone:   make(chan struct{}),
		terminated: make(chan struct{}),
	}

	// self = globalThis inside the worker (DedicatedWorkerGlobalScope).
	if _, err := ctx.RunScript("var self = globalThis;", "worker-scope.js"); err != nil {
		iso.Dispose()
		return nil, fmt.Errorf("worker scope init failed: %w", err)
	}

	// worker→parent native post: the JS side passes an already-stringified
	// JSON payload; queue it for parent delivery.
	nativePost := func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		payload := "null"
		if len(info.Args()) > 0 {
			payload = info.Args()[0].String()
		}
		msg, _ := json.Marshal(workerMessage{Type: "message", Payload: json.RawMessage(payload)})
		select {
		case w.outbound <- string(msg):
		default:
			log.Printf("[sandbox] worker %d outbound queue full, message dropped", w.id)
		}
		return nil
	}
	if fn := v8go.NewFunctionTemplate(iso, nativePost).GetFunction(ctx); fn != nil {
		w.global().Set("__besWorkerNativePost", fn)
	}

	// worker-side close(): notify the loop to exit via the outbound queue.
	nativeClose := func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		select {
		case w.outbound <- `{"type":"close"}`:
		default:
		}
		return nil
	}
	if fn := v8go.NewFunctionTemplate(iso, nativeClose).GetFunction(ctx); fn != nil {
		w.global().Set("__besWorkerClose", fn)
	}

	if err := w.injectWorkerEnv(); err != nil {
		iso.Dispose()
		return nil, err
	}

	// Run the worker source on the worker thread. Script errors are reported
	// to the parent as an error envelope; the worker does not stay alive.
	if _, err := ctx.RunScript(source, "worker.js"); err != nil {
		errMsg, _ := json.Marshal(workerMessage{Type: "error", Payload: json.RawMessage(jsonString(err.Error()))})
		select {
		case w.outbound <- string(errMsg):
		default:
		}
		w.terminate()
		return nil, fmt.Errorf("worker script failed: %w", err)
	}

	// Start the worker loop (the worker thread). The outbound pump is started
	// by injectWorkerConstructor once the parent context is available.
	go w.workerLoop()

	return w, nil
}

// injectWorkerEnv installs the worker-side environment: navigator (UA from
// the session fingerprint), timers, console, and the worker JS shim
// (postMessage/onmessage surface, base64, scope flags).
func (w *Worker) injectWorkerEnv() error {
	iso, ctx := w.iso, w.ctx

	// navigator with the fingerprint UA (workers have a navigator object but
	// no DOM; signing scripts commonly read navigator.userAgent).
	if w.userAgent != "" {
		navTmpl := v8go.NewObjectTemplate(iso)
		navTmpl.Set("userAgent", w.userAgent)
		navTmpl.Set("hardwareConcurrency", int32(8))
		if navVal, err := navTmpl.NewInstance(ctx); err == nil && navVal != nil {
			w.global().Set("navigator", navVal)
		}
	}

	// Timers inside the worker: own manager, drained by the worker loop.
	tm := w.timers
	setFn := func(cb v8go.FunctionCallback, names ...string) {
		if fn := v8go.NewFunctionTemplate(iso, cb).GetFunction(ctx); fn != nil {
			for _, n := range names {
				w.global().Set(n, fn)
			}
		}
	}
	setFn(tm.SetTimeoutCallback(iso), "setTimeout")
	setFn(tm.SetIntervalCallback(iso), "setInterval")
	setFn(tm.ClearTimeoutCallback(iso), "clearTimeout", "clearInterval")

	// console → session sink (or stdout). Per-level Go fns are wired into a
	// real console object by the JS shim below.
	for _, level := range []string{"log", "info", "warn", "error", "debug"} {
		lvl := level
		cb := func(info *v8go.FunctionCallbackInfo) *v8go.Value {
			parts := make([]string, 0, len(info.Args()))
			for _, a := range info.Args() {
				parts = append(parts, a.String())
			}
			msg := joinStrings(parts, " ")
			if w.console != nil {
				w.console.Write(lvl, msg)
			} else {
				log.Printf("[worker console] %s: %s", lvl, msg)
			}
			return nil
		}
		if fn := v8go.NewFunctionTemplate(iso, cb).GetFunction(ctx); fn != nil {
			w.global().Set("console_"+level, fn)
		}
	}

	// Inject the worker JS surface.
	if _, err := ctx.RunScript(workerEnvJS, "worker-env.js"); err != nil {
		return fmt.Errorf("worker env shim failed: %w", err)
	}
	return nil
}

// global returns the worker's global object (worker thread only).
func (w *Worker) global() *v8go.Object {
	return w.ctx.Global()
}

// terminate stops the worker loop, waits for it to exit, then disposes the
// isolate. Idempotent. Disposing while the loop is inside a V8 call is a
// use-after-free, so the wait is mandatory.
func (w *Worker) terminate() {
	w.once.Do(func() {
		close(w.stop)
		w.timers.StopAll()
		// Wake the loop if it is blocked on the mailbox.
		select {
		case w.inbound <- `{"type":"close"}`:
		default:
		}
		select {
		case <-w.loopDone:
		case <-time.After(2 * time.Second):
			log.Printf("[sandbox] worker %d loop did not exit within 2s; disposing anyway", w.id)
		}
		w.iso.Dispose()
		close(w.terminated)
	})
}

// workerLoop is the worker thread's event loop. It owns the worker Isolate:
// every V8 access for this worker happens on this goroutine (or during
// StartWorker before the loop starts).
func (w *Worker) workerLoop() {
	defer close(w.loopDone)
	for {
		select {
		case <-w.stop:
			return
		case raw := <-w.inbound:
			var msg workerMessage
			if err := json.Unmarshal([]byte(raw), &msg); err != nil {
				continue
			}
			switch msg.Type {
			case "message":
				payload := "null"
				if len(msg.Payload) > 0 {
					payload = string(msg.Payload)
				}
				w.dispatchMessage(payload)
			case "close":
				return
			}
		}
	}
}

// dispatchMessage runs self.__besWorkerDeliver on the worker thread, then
// drains worker timers + microtasks so postMessage handlers that schedule
// setTimeout work settle before the next message.
func (w *Worker) dispatchMessage(payloadJSON string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[sandbox] worker dispatch panic: %v", r)
		}
	}()
	if _, err := w.ctx.RunScript(
		fmt.Sprintf("typeof __besWorkerDeliver === 'function' && __besWorkerDeliver(%s)", jsonString(payloadJSON)),
		"worker-deliver.js"); err != nil {
		log.Printf("[sandbox] worker deliver error: %v", err)
	}
	for i := 0; i < 20; i++ {
		w.timers.DrainCallbacks()
		w.timers.Flush(50 * time.Millisecond)
		w.ctx.PerformMicrotaskCheckpoint()
	}
}

// joinStrings joins with a separator (avoids importing strings for one use).
func joinStrings(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

// workerEnvJS is the worker-side JS surface. Precondition: the Go side has
// injected __besWorkerNativePost(payloadJSON), __besWorkerClose(), and
// console_<level> functions.
const workerEnvJS = `
(function(){
  'use strict';
  // console object backed by the Go-injected per-level functions.
  function mk(level){ return function(){ var a=[].slice.call(arguments).map(String).join(' '); self['console_'+level](a); }; }
  self.console = {
    log: mk('log'), info: mk('info'), warn: mk('warn'),
    error: mk('error'), debug: mk('debug')
  };

  // __besWorkerDeliver(payloadJSON): called by the Go worker loop for each
  // parent→worker message; dispatches to onmessage/onerror.
  self.__besWorkerDeliver = function(payloadJSON) {
    var data = JSON.parse(payloadJSON);
    if (typeof self.onmessage === 'function') {
      try { self.onmessage({ data: data }); } catch (e) {
        if (typeof self.onerror === 'function') self.onerror({ message: String(e) });
      }
    }
  };

  // postMessage(data): worker → parent. Payload is JSON-stringified here so
  // the Go callback only handles strings (constraint #14).
  self.postMessage = function(data) {
    __besWorkerNativePost(JSON.stringify(data === undefined ? null : data));
  };

  // close(): ends the worker.
  self.close = function() { __besWorkerClose(); };

  // Base64 helpers (same algorithms as the parent shim).
  var chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
  var lookup = {};
  for (var i = 0; i < chars.length; i++) lookup[chars[i]] = i;
  self.atob = function(str) {
    var output = '', buffer = 0, bits = 0;
    str = String(str).replace(/[^A-Za-z0-9+/=]/g, '');
    for (var i = 0; i < str.length; i++) {
      var c = str[i];
      if (c === '=') break;
      buffer = (buffer << 6) | lookup[c];
      bits += 6;
      if (bits >= 8) { bits -= 8; output += String.fromCharCode((buffer >> bits) & 0xFF); }
    }
    return output;
  };
  self.btoa = function(str) {
    var output = '', buffer = 0, bits = 0;
    for (var i = 0; i < str.length; i++) {
      buffer = (buffer << 8) | str.charCodeAt(i);
      bits += 8;
      while (bits >= 6) { bits -= 6; output += chars[(buffer >> bits) & 0x3F]; }
    }
    if (bits > 0) output += chars[(buffer << (6 - bits)) & 0x3F];
    while (output.length % 4) output += '=';
    return output;
  };

  // Worker-visible scope flags: DedicatedWorkerGlobalScope has no DOM.
  self.window = undefined;
  self.document = undefined;
})();
`

// injectWorkerConstructor replaces the env_shim Worker stub with a real
// implementation: each `new Worker(source)` spawns a dedicated isolate
// running the source, with postMessage/onmessage/terminate bridged over
// JSON envelopes. Called after PostContextBuilder.Build() so the shim stub
// is overwritten.
func injectWorkerConstructor(p *PostContextBuilder, sess *Session) {
	iso, ctx := p.iso, p.ctx
	parentTimers := p.timerMgr

	// __besWorkerCreate(source) → worker id (number). Runs on the parent
	// isolate thread (plain FunctionCallback); the new worker's isolate is
	// created here but only touched from its own goroutine afterwards.
	createCb := func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		source := ""
		if args := info.Args(); len(args) > 0 {
			source = args[0].String()
		}
		w, err := StartWorker(source, parentTimers, sess.consoleSink, p.userAgent())
		if err != nil {
			log.Printf("[sandbox] worker create failed: %v", err)
			// Return -1; the JS wrapper fires onerror.
			v, _ := v8go.NewValue(iso, int32(-1))
			return v
		}
		id := sess.workers.register(w)
		w.startParentPump(parentTimers, p.ctx)
		v, _ := v8go.NewValue(iso, id)
		return v
	}
	if createFn := v8go.NewFunctionTemplate(iso, createCb).GetFunction(ctx); createFn != nil {
		p.global.Set("__besWorkerCreate", createFn)
	}

	// __besWorkerPost(id, payloadJSON): parent → worker mailbox.
	postCb := func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		if len(info.Args()) < 2 {
			return nil
		}
		id := info.Args()[0].Int32()
		payload := info.Args()[1].String()
		if w := sess.workers.get(id); w != nil {
			msg, _ := json.Marshal(workerMessage{Type: "message", Payload: json.RawMessage(payload)})
			select {
			case w.inbound <- string(msg):
			default:
				log.Printf("[sandbox] worker %d inbound queue full, message dropped", id)
			}
		}
		return nil
	}
	if fn := v8go.NewFunctionTemplate(iso, postCb).GetFunction(ctx); fn != nil {
		p.global.Set("__besWorkerPost", fn)
	}

	// __besWorkerTerminate(id): stop the worker and dispose its isolate.
	termCb := func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		if len(info.Args()) > 0 {
			sess.workers.remove(info.Args()[0].Int32())
		}
		return nil
	}
	if fn := v8go.NewFunctionTemplate(iso, termCb).GetFunction(ctx); fn != nil {
		p.global.Set("__besWorkerTerminate", fn)
	}

	// JS Worker class: real semantics over the Go bridges.
	workerJS := `
	(function(){
	  'use strict';
	  window.Worker = function(url, options) {
	    var source = '';
	    if (typeof url === 'string') {
	      if (/^blob:/.test(url) && typeof URL !== 'undefined' && URL.__besBlobs) {
	        source = URL.__besBlobs[url] || '';
	      } else {
	        source = url; // treat as inline source (eval-style worker)
	      }
	    } else if (url && typeof url === 'object') {
	      source = (typeof URL !== 'undefined' && URL.__besBlobs ? URL.__besBlobs[url.url] : '') || '';
	    }
	    var selfObj = this;
	    this.__besID = __besWorkerCreate(source);
	    this.onmessage = null;
	    this.onerror = null;
	    this.onmessageerror = null;
	    this.readyState = this.__besID >= 0 ? 1 : 0; // 1=running
	    if (this.__besID >= 0) {
	      __besWorkerRegisterReceiver(this.__besID, this);
	    }

	    this.postMessage = function(data) {
	      if (selfObj.__besID < 0) return;
	      __besWorkerPost(selfObj.__besID, JSON.stringify(data === undefined ? null : data));
	    };
	    this.terminate = function() {
	      if (selfObj.__besID < 0) return;
	      __besWorkerUnregisterReceiver(selfObj.__besID);
	      __besWorkerTerminate(selfObj.__besID);
	      selfObj.__besID = -1;
	      selfObj.readyState = 3;
	    };
	    this.addEventListener = function(type, fn) {
	      if (type === 'message') { selfObj.onmessage = fn; }
	      else if (type === 'error') { selfObj.onerror = fn; }
	    };
	    this.removeEventListener = function() {};
	    this.dispatchEvent = function() { return true; };

	    if (this.__besID < 0) {
	      var self = this;
	      setTimeout(function(){ if (self.onerror) self.onerror({ message: 'worker failed to start' }); }, 0);
	    }
	  };
	  window.Worker.prototype.constructor = window.Worker;
	})();
	`
	if _, err := ctx.RunScript(workerJS, "worker-class.js"); err != nil {
		log.Printf("[sandbox] worker class warning: %v", err)
	}

	// Parent-side dispatcher: JS Worker objects register a receiver that the
	// outbound pump's parent-thread callback invokes. Map: worker id → object.
	if _, err := ctx.RunScript(`
		(function(){
		  'use strict';
		  var receivers = {};
		  window.__besWorkerOnParentMessage = function(id, type, payloadJSON) {
		    var recv = receivers[id];
		    if (!recv) return;
		    try {
		      var data = JSON.parse(payloadJSON);
		      if (type === 'error') {
		        if (typeof recv.onerror === 'function') recv.onerror({ message: String(data) });
		      } else if (typeof recv.onmessage === 'function') {
		        recv.onmessage({ data: data });
		      }
		    } catch (e) {
		      if (typeof recv.onerror === 'function') recv.onerror({ message: String(e) });
		    }
		  };
		  window.__besWorkerRegisterReceiver = function(id, workerObj) { receivers[id] = workerObj; };
		  window.__besWorkerUnregisterReceiver = function(id) { delete receivers[id]; };
		})();
	`, "worker-parent-bridge.js"); err != nil {
		log.Printf("[sandbox] worker parent bridge warning: %v", err)
	}
}

// startParentPump forwards worker→parent envelopes onto the parent isolate
// thread. It runs on its own goroutine but never touches V8 directly: each
// envelope becomes a timer-queue callback carrying the parent context, and
// that callback executes during EvalAwait's drain on the parent isolate
// thread.
func (w *Worker) startParentPump(parentTimers *TimerManager, parentCtx *v8go.Context) {
	wRef := w
	go func() {
		for {
			select {
			case <-wRef.stop:
				return
			case raw, ok := <-wRef.outbound:
				if !ok {
					return
				}
				var msg workerMessage
				if err := json.Unmarshal([]byte(raw), &msg); err != nil {
					continue
				}
				switch msg.Type {
				case "message", "error":
					parentTimers.scheduleTimer(0, false, func() {
						// Runs on the parent isolate thread via the timer drain.
						payload := "null"
						if len(msg.Payload) > 0 {
							payload = string(msg.Payload)
						}
						call := fmt.Sprintf(
							"typeof __besWorkerOnParentMessage === 'function' && __besWorkerOnParentMessage(%d, %q, %s)",
							wRef.id, msg.Type, jsonString(payload))
						if _, err := parentCtx.RunScript(call, "worker-parent-deliver.js"); err != nil {
							log.Printf("[sandbox] worker parent deliver error: %v", err)
						}
					})
				case "close":
					return
				}
			}
		}
	}()
}
