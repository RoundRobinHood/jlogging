package jlogging

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLog carries information about a request and its resolution.
// This package directly marshals this struct.
// RequestLog fields are safe to read and edit in normal execution flow of gin handlers (this fact is useful for the Details map)
type RequestLog struct {
	URI            string        `json:"uri"`
	Method         string        `json:"method"`
	ResponseStatus int           `json:"status"`
	RequestTime    time.Time     `json:"time"`
	Duration       int64         `json:"duration"`
	ClientIP       string        `json:"ip"`
	Logs           []string      `json:"logs"`
	Details        gin.H         `json:"details"`
	Error          *PanicDetails `json:"error"`
}

type PanicDetails struct {
	Descriptor  any    `json:"desc"`
	PriorStatus int    `json:"oldStatus"`
	StackTrace  string `json:"stackTrace"`
}

// Printf prints adds a formatted string to the RequestLog's Logs, formatted via fmt.Sprintf (and creates RequestLog.Logs if it's nil)
// Usage is suggested as progression markers in processing, similar to normal logging (as the log array can indicate the code route and help with debugging)
// For logging more structured information, (*RequestLog).Set is recommended (as it formats directly into the resulting JSON and can more easily be processed)
func (r *RequestLog) Printf(format string, values ...any) {
	text := fmt.Sprintf(format, values...)
	if r.Logs == nil {
		r.Logs = make([]string, 1)
	}

	r.Logs = append(r.Logs, text)
}

// Set sets a key in the Details object, and creates it if it doesn't exist (helps avoid sets on Details as a nil map)
// Use this to log information essential to the intention of the request, or things figured out during processing.
// For example, it can store auth results (like why auth was denied / approved, or who the requester was approved as)
// Or it can store processing results halfway (especially if computation is complex), allowing deferred debugging to walk through the process more effectively.
func (r *RequestLog) Set(key string, value any) {
	if r.Details == nil {
		r.Details = make(map[string]any)
	}

	r.Details[key] = value
}

func MarshalWithFallback(l *RequestLog) ([]byte, error) {
	bytes, err := json.MarshalIndent(l, "", "    ")
	if err != nil {
		l.Details = nil
		l.Printf("JLogging: Failed to marshal due to problem with something in Details object")
		b, err := json.MarshalIndent(l, "", "    ")
		if err != nil {
			return nil, err
		}
		bytes = b
	}

	return bytes, nil
}

// Middleware returns a gin compatible middleware.
// The middleware logs a JSON object per request via fmt.Printf, and also recovers panics
// The RequestLog object can be accessed and edited in handlers via c.Get("jrl")
// When panics are recovered, the result has non-null "error", and status is set to 500
// If the before-panic status is needed in the logged object, it can be found in "error.oldStatus"
// If Marshal fails due to bad Details values (such as circular references), Details is set to nil and a log is added (see MarshalWithFallback)
// If failures occur due to bugs in jlogging (marshal failing on a Details-nil RequestLog object), the logged object has only "jlog" set with a descriptor and the error
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		l := &RequestLog{
			URI:         c.Request.URL.Path,
			Method:      c.Request.Method,
			RequestTime: start,
			ClientIP:    c.ClientIP(),
			Logs:        make([]string, 0),
			Details:     make(gin.H),
		}
		defer func() {
			if r := recover(); r != nil {
				stackTrace := debug.Stack()
				l.Duration = time.Since(start).Milliseconds()
				priorStatus := c.Writer.Status()
				c.AbortWithStatusJSON(500, gin.H{"error": "Internal error"})

				l.ResponseStatus = 500
				l.Error = &PanicDetails{
					PriorStatus: priorStatus,
					Descriptor:  r,
					StackTrace:  string(stackTrace),
				}

				bytes, err := MarshalWithFallback(l)
				if err != nil {
					fmt.Printf("{\"jlog\":\"Could not marshal request log during panic: %s\"}\n", err)
				} else {
					fmt.Printf("%s\n", string(bytes))
				}
			}
		}()

		c.Set("jrl", l)

		c.Next()

		l.Duration = time.Since(start).Milliseconds()
		l.ResponseStatus = c.Writer.Status()

		bytes, err := MarshalWithFallback(l)
		if err != nil {
			fmt.Printf("{\"jlog\":\"Could not marshal request log: %s\"}\n", err)
		} else {
			fmt.Printf("%s\n", string(bytes))
		}
	}
}
