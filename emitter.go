package main

import (
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"sync"
	"time"

	"github.com/buger/goreplay/proto"

	"github.com/buger/goreplay/byteutils"
)

// Emitter represents an abject to manage plugins communication
type Emitter struct {
	sync.WaitGroup
	plugins *InOutPlugins
}

// NewEmitter creates and initializes new Emitter object.
func NewEmitter() *Emitter {
	return &Emitter{}
}

// Start initialize loop for sending data from inputs to outputs
func (e *Emitter) Start(plugins *InOutPlugins, middlewareCmd string) {
	if Settings.CopyBufferSize < 1 {
		Settings.CopyBufferSize = 5 << 20
	}
	e.plugins = plugins

	if middlewareCmd != "" {
		middleware := NewMiddleware(middlewareCmd)

		for _, in := range plugins.Inputs {
			middleware.ReadFrom(in)
		}

		e.plugins.Inputs = append(e.plugins.Inputs, middleware)
		e.plugins.All = append(e.plugins.All, middleware)
		e.Add(1)
		go func() {
			defer e.Done()
			if err := CopyMulty(middleware, plugins.Outputs...); err != nil {
				Debug(2, fmt.Sprintf("[EMITTER] error during copy: %q", err))
			}
		}()
	} else {
		for _, in := range plugins.Inputs {
			e.Add(1)
			go func(in PluginReader) {
				defer e.Done()
				if err := CopyMulty(in, plugins.Outputs...); err != nil {
					Debug(2, fmt.Sprintf("[EMITTER] error during copy: %q", err))
				}
			}(in)
		}
	}
}

// Close closes all the goroutine and waits for it to finish.
func (e *Emitter) Close() {
	for _, p := range e.plugins.All {
		if cp, ok := p.(io.Closer); ok {
			cp.Close()
		}
	}
	if len(e.plugins.All) > 0 {
		// wait for everything to stop
		e.Wait()
	}
	e.plugins.All = nil // avoid Close to make changes again
}

// CopyMulty copies from 1 reader to multiple writers
func CopyMulty(src PluginReader, writers ...PluginWriter) error {
	wIndex := 0
	modifier := NewHTTPModifier(&Settings.ModifierConfig)
	// filteredRequests := make(map[string]int64)
	allowedRequests := make(map[string]int64)
	filteredRequestsLastCleanTime := time.Now().UnixNano()
	filteredCount := 0

	for {
		msg, err := src.PluginRead()
		if err != nil {
			if err == ErrorStopped || err == io.EOF {
				return nil
			}
			return err
		}
		if msg != nil && len(msg.Data) > 0 {
			if len(msg.Data) > int(Settings.CopyBufferSize) {
				msg.Data = msg.Data[:Settings.CopyBufferSize]
			}
			meta := payloadMeta(msg.Meta)
			if len(meta) < 3 {
				Debug(2, fmt.Sprintf("[EMITTER] Found malformed record %q from %q", msg.Meta, src))
				continue
			}
			requestID := byteutils.SliceToString(meta[1])
			// start a subroutine only when necessary
			if Settings.Verbose >= 3 {
				Debug(3, "[EMITTER] input: ", byteutils.SliceToString(msg.Meta[:len(msg.Meta)-1]), " from: ", src)
			}
			if isRequestPayload(msg.Meta) {
				if !proto.HasRequestTitle(msg.Data) {
					continue
				}

				// Handle request processing
				if modifier != nil {
					Debug(3, "[EMITTER] modifier:", requestID, "from:", src)
					msg.Data = modifier.Rewrite(msg.Data)
					// If modifier tells to skip request
					if len(msg.Data) == 0 {
						continue
					}
					Debug(3, "[EMITTER] Rewritten input:", requestID, "from:", src)
				}
				// Request passed all filters, mark it as allowed only if trackResponse is enabled
				if Settings.TrackResponse {
					allowedRequests[requestID] = time.Now().UnixNano()
					filteredCount++
				}
			} else {
				if !Settings.TrackResponse {
					continue
				}

				// For responses, check if corresponding request was allowed
				if _, ok := allowedRequests[requestID]; !ok {
					// Response for unallowed request, skip it
					continue
				}
				// Clean up the allowed request entry
				delete(allowedRequests, requestID)
			}

			if Settings.PrettifyHTTP {
				msg.Data = prettifyHTTP(msg.Data)
				if len(msg.Data) == 0 {
					continue
				}
			}

			if Settings.SplitOutput {
				if Settings.RecognizeTCPSessions {
					if !PRO {
						log.Fatal("Detailed TCP sessions work only with PRO license")
					}
					hasher := fnv.New32a()
					hasher.Write(meta[1])

					wIndex = int(hasher.Sum32()) % len(writers)
					if _, err := writers[wIndex].PluginWrite(msg); err != nil {
						return err
					}
				} else {
					// Simple round robin
					if _, err := writers[wIndex].PluginWrite(msg); err != nil {
						return err
					}

					wIndex = (wIndex + 1) % len(writers)
				}
			} else {
				for _, dst := range writers {
					if _, err := dst.PluginWrite(msg); err != nil && err != io.ErrClosedPipe {
						return err
					}
				}
			}
		}

		// Run GC on each 1000 request
		if filteredCount > 0 && filteredCount%1000 == 0 {
			// Clean up filtered requests for which we didn't get a response to filter
			now := time.Now().UnixNano()
			if now-filteredRequestsLastCleanTime > int64(60*time.Second) {
				// Also clean up old allowed requests that didn't get responses
				for k, v := range allowedRequests {
					if now-v > int64(60*time.Second) {
						delete(allowedRequests, k)
					}
				}
				filteredRequestsLastCleanTime = time.Now().UnixNano()
			}
		}
	}
}
