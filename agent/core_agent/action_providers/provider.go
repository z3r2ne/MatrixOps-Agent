// Package actionproviders provides ActionProvider implementations for the
// core_agent package.
//
// The ActionProvider interface itself is defined in the coreagent package
// (to avoid an import cycle) and registered via
// coreagent.RegisterActionProviderFactory in this package's init().
//
// Two primary implementations exist:
//
//   - CompatibleActionProvider: uses the generic ChatClient (stream_v2.go)
//     and parses JSON tool envelopes from the model text stream.
//
//   - OpenAINativeActionProvider: uses the official openai-go SDK
//     (stream_openai_native.go) with native function/tool_calls.
package actionproviders
