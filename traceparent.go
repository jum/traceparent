// Package traceparent implements a simple middleware to parse the
// HTTP traceparent header [W3C] into its parts without using OTEL or
// other big tracing packages. The trace context will be output via
// [log/slog] if using the log functions that inclue a [context]
// parameter as the first argument.
//
// [W3C]: https://www.w3.org/TR/trace-context/
package traceparent

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/jussi-kalliokoski/slogdriver"
)

// Traceparent implements a middleware function that will inject the
// [slogdriver.Trace] structure into the current requests context. To
// make this context available to the [log/slog] logging functions, be
// sure to the the variants including a [context] argument.
func Traceparent(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		traceparent := strings.Split(r.Header.Get("traceparent"), "-")
		if len(traceparent) == 4 && traceparent[0] == "00" {
			flags, err := strconv.ParseInt(traceparent[3], 16, 8)
			if err == nil {
				trace := slogdriver.Trace{
					ID:      traceparent[1],
					SpanID:  traceparent[2],
					Sampled: (flags & 1) != 0,
				}
				ctx := trace.Context(r.Context())
				next.ServeHTTP(w, r.WithContext(ctx))
			} else {
				next.ServeHTTP(w, r)
			}
		} else {
			next.ServeHTTP(w, r)
		}
	}
	return http.HandlerFunc(fn)
}
