package layout

import (
	"fmt"
	"io"
)

type Message struct {
	Role string
	Text string
}

type MessageRenderer struct {
	history []Message
}

func (r *MessageRenderer) Add(role, text string) {
	if text == "" {
		return
	}
	r.history = append(r.history, Message{Role: role, Text: text})
}

func (r *MessageRenderer) History() []Message {
	out := make([]Message, len(r.history))
	copy(out, r.history)
	return out
}

func (r *MessageRenderer) RenderLast(w io.Writer) {
	if len(r.history) == 0 {
		return
	}
	msg := r.history[len(r.history)-1]
	fmt.Fprintf(w, "%s:\n%s\n\n", msg.Role, msg.Text)
}

type InputRenderer struct{}

func (InputRenderer) RenderPrompt(w io.Writer) {
	fmt.Fprint(w, "> ")
}
