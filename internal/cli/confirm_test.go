package cli

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

// runCLISplit runs the root command like runCLI but keeps stdout and stderr
// in separate buffers, for tests that care which stream a message lands on.
func runCLISplit(t *testing.T, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := NewRootCmd()
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errOut.String(), err
}

func TestConfirm_AnswersAndStreams(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"y\n", true},
		{"YES\n", true},
		{" yes ", true},
		{"n\n", false},
		{"\n", false},
		{"", false},
		{"yep\n", false},
	} {
		cmd := NewRootCmd()
		out, errOut := new(bytes.Buffer), new(bytes.Buffer)
		cmd.SetOut(out)
		cmd.SetErr(errOut)
		cmd.SetIn(strings.NewReader(tc.in))
		if got := confirm(cmd, "Proceed?"); got != tc.want {
			t.Errorf("confirm(%q) = %v, want %v", tc.in, got, tc.want)
		}
		if out.Len() != 0 {
			t.Errorf("confirm(%q) wrote to stdout: %q", tc.in, out.String())
		}
		wantContains(t, errOut.String(), "Proceed? [y/N] ")
		if aborted := strings.Contains(errOut.String(), "Aborted."); aborted == tc.want {
			t.Errorf("confirm(%q): Aborted. present = %v", tc.in, aborted)
		}
	}
}

func TestDeletePrompts_GoToStderr(t *testing.T) {
	api := newFakeAPI(t)
	api.json("DELETE /api/documents/doc-1", http.StatusOK, map[string]any{"is_deleted": true})
	api.json("DELETE /api/documents/doc-1/attachments/att-1", http.StatusOK, map[string]any{"is_deleted": true})
	useEnvKey(t, goodKey)

	stdout, stderr, err := runCLISplit(t, "y\n", "document", "attachment", "delete", "doc-1", "att-1", "--json")
	if err != nil {
		t.Fatalf("attachment delete failed: %v", err)
	}
	if decodeJSON(t, stdout)["deleted"] != true {
		t.Errorf("unexpected JSON: %s", stdout)
	}
	wantContains(t, stderr, "Delete attachment att-1 from document doc-1? [y/N]")

	// Text mode: prompt on stderr, result on stdout.
	stdout, stderr, err = runCLISplit(t, "y\n", "document", "delete", "doc-1")
	if err != nil {
		t.Fatalf("document delete failed: %v", err)
	}
	wantContains(t, stderr, "Delete document doc-1? [y/N]")
	wantContains(t, stdout, "Document deleted.")
	if strings.Contains(stdout, "[y/N]") {
		t.Errorf("prompt leaked to stdout: %q", stdout)
	}

	// Aborting with --json leaves stdout empty.
	stdout, stderr, err = runCLISplit(t, "n\n", "document", "delete", "doc-1", "--json")
	if err != nil {
		t.Fatalf("aborted delete failed: %v", err)
	}
	if stdout != "" {
		t.Errorf("aborted --json delete wrote to stdout: %q", stdout)
	}
	wantContains(t, stderr, "Aborted.")
}
