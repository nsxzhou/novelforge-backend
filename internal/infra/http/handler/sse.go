package handler

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/sse"
)

// writeSSEStream consumes a schema.StreamReader and writes SSE events to the response
// using Hertz's built-in SSE writer. When the stream is fully consumed, onDone is called
// with the accumulated content. The returned value from onDone is sent as the "done" event data.
func writeSSEStream(ctx *app.RequestContext, stream *schema.StreamReader[*schema.Message], onDone func(content string) (any, error)) {
	w := sse.NewWriter(ctx)
	ctx.Response.Header.Set("X-Accel-Buffering", "no")
	defer func() {
		_ = w.Close()
	}()
	defer stream.Close()

	var contentBuilder strings.Builder

	for {
		msg, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			// Stream error - send error event and close.
			_ = w.Write(&sse.Event{
				Type: "error",
				Data: []byte(err.Error()),
			})
			if onDone != nil {
				_, _ = onDone("")
			}
			return
		}
		if msg == nil {
			continue
		}

		chunk := msg.Content
		if chunk == "" {
			continue
		}

		contentBuilder.WriteString(chunk)
		_ = w.Write(&sse.Event{
			Type: "content",
			Data: []byte(chunk),
		})
	}

	fullContent := contentBuilder.String()
	if onDone != nil {
		result, err := onDone(fullContent)
		if err != nil {
			_ = w.Write(&sse.Event{
				Type: "error",
				Data: []byte(err.Error()),
			})
			return
		}
		data, jsonErr := json.Marshal(result)
		if jsonErr != nil {
			_ = w.Write(&sse.Event{
				Type: "error",
				Data: []byte(jsonErr.Error()),
			})
			return
		}
		_ = w.Write(&sse.Event{
			Type: "done",
			Data: data,
		})
	} else {
		_ = w.Write(&sse.Event{
			Type: "done",
			Data: []byte(fullContent),
		})
	}
}
