package sandbox

// ConsoleSink receives console.* messages emitted from inside a sandbox
// session.
//
// Write is invoked from V8 function-callback threads during script execution,
// so implementations MUST be non-blocking and safe for concurrent use. A
// blocking Write would stall the V8 isolate (and the goroutine running the
// script) and can deadlock the whole session.
type ConsoleSink interface {
	Write(level, message string)
}

// ConsoleSinkFactory returns a fresh ConsoleSink for a newly created session.
//
// It is called once per Engine.CreateSession, so each session routes its
// console output to an independent sink (and thus its own stream of
// subscribers) without cross-session bleed. This indirection keeps the
// sandbox package decoupled from any concrete streaming/broadcaster
// implementation — the bridge layer supplies the factory.
type ConsoleSinkFactory func() ConsoleSink
