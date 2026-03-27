package output

import "encoding/json"

// JSON encodes v as JSON to the renderer's writer.
// Pretty-prints with indentation when outputting to a TTY, compact otherwise.
func (r *Renderer) JSON(v any) error {
	if r.quiet {
		return nil
	}
	enc := json.NewEncoder(r.w)
	if r.isTTY {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}

// JSONError writes a structured JSON error to the renderer's writer.
func (r *Renderer) JSONError(err error, code int) error {
	payload := struct {
		Error string `json:"error"`
		Code  int    `json:"code"`
	}{
		Error: err.Error(),
		Code:  code,
	}
	enc := json.NewEncoder(r.w)
	if r.isTTY {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(payload)
}
