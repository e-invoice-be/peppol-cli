package output

import (
	"context"
	"fmt"
	"io"
	"os"
)

// KVPair represents a key-value pair for display.
type KVPair struct {
	Key   string
	Value string
}

// Renderer dispatches output to the correct format (text/JSON/quiet).
type Renderer struct {
	w       io.Writer
	isJSON  bool
	quiet   bool
	noColor bool
	isTTY   bool
	color   bool
}

// NewRenderer creates a Renderer that auto-detects TTY from stdout.
func NewRenderer(w io.Writer, jsonMode, quiet, noColor bool) *Renderer {
	tty := IsTTY(os.Stdout.Fd())
	r := &Renderer{
		w:       w,
		isJSON:  jsonMode,
		quiet:   quiet,
		noColor: noColor,
		isTTY:   tty,
		color:   hasColor(noColor, tty),
	}
	return r
}

// NewTestRenderer creates a Renderer with explicit TTY control for testing.
func NewTestRenderer(w io.Writer, jsonMode, quiet, noColor, isTTY bool) *Renderer {
	return &Renderer{
		w:       w,
		isJSON:  jsonMode,
		quiet:   quiet,
		noColor: noColor,
		isTTY:   isTTY,
		color:   hasColor(noColor, isTTY),
	}
}

// Writer returns the underlying writer.
func (r *Renderer) Writer() io.Writer { return r.w }

// IsJSON reports whether JSON output mode is active.
func (r *Renderer) IsJSON() bool { return r.isJSON }

// IsQuiet reports whether quiet mode is active.
func (r *Renderer) IsQuiet() bool { return r.quiet }

// KeyValue renders key-value pairs with aligned keys.
func (r *Renderer) KeyValue(pairs []KVPair) error {
	if r.quiet {
		return nil
	}

	// Find max key width for alignment.
	maxWidth := 0
	for _, p := range pairs {
		if len(p.Key) > maxWidth {
			maxWidth = len(p.Key)
		}
	}

	for _, p := range pairs {
		label := fmt.Sprintf("%-*s", maxWidth, p.Key)
		if r.color {
			label = LabelStyle.Render(label)
		}
		fmt.Fprintf(r.w, "%s  %s\n", label, p.Value)
	}
	return nil
}

// Success prints a success message.
func (r *Renderer) Success(msg string) {
	if r.quiet {
		return
	}
	if r.color {
		msg = SuccessStyle.Render(msg)
	}
	fmt.Fprintln(r.w, msg)
}

// Error prints an error message. Errors are always shown, even in quiet mode.
func (r *Renderer) Error(msg string) {
	if r.color {
		msg = ErrorStyle.Render(msg)
	}
	fmt.Fprintln(r.w, msg)
}

// Context plumbing.

type rendererKey struct{}

// WithRenderer stores a Renderer in the context.
func WithRenderer(ctx context.Context, r *Renderer) context.Context {
	return context.WithValue(ctx, rendererKey{}, r)
}

// FromContext retrieves the Renderer from the context.
// Returns a default stdout renderer if none is found.
func FromContext(ctx context.Context) *Renderer {
	if ctx == nil {
		return defaultRenderer()
	}
	if r, ok := ctx.Value(rendererKey{}).(*Renderer); ok {
		return r
	}
	return defaultRenderer()
}

func defaultRenderer() *Renderer {
	return NewRenderer(os.Stdout, false, false, false)
}
