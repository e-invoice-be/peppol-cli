package version

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// go install only works when go.mod declares the GitHub repository path.
const canonicalModule = "github.com/e-invoice-be/peppol-cli"

const versionPkg = canonicalModule + "/internal/version"

var ldflagVars = []string{"Version", "Commit", "Date"}

func TestModulePath(t *testing.T) {
	t.Run("go.mod", func(t *testing.T) {
		m := regexp.MustCompile(`(?m)^module\s+(\S+)`).FindStringSubmatch(readFile(t, "../../go.mod"))
		if m == nil {
			t.Fatal("no module directive in go.mod")
		}
		if m[1] != canonicalModule {
			t.Errorf("go.mod module = %q, want %q", m[1], canonicalModule)
		}
	})

	t.Run("README", func(t *testing.T) {
		want := "go install " + canonicalModule + "/cmd/peppol@"
		if !strings.Contains(readFile(t, "../../README.md"), want) {
			t.Errorf("README.md has no %q line", want)
		}
	})
}

func TestLdflagsTargetVersionPackage(t *testing.T) {
	re := regexp.MustCompile(`-X\s+(\S+)\.(` + strings.Join(ldflagVars, "|") + `)=`)
	for _, file := range []string{"../../Makefile", "../../.goreleaser.yml"} {
		seen := map[string]bool{}
		for _, m := range re.FindAllStringSubmatch(readFile(t, file), -1) {
			seen[m[2]] = true
			if m[1] != versionPkg {
				t.Errorf("%s: -X %s.%s, want package %s", file, m[1], m[2], versionPkg)
			}
		}
		for _, v := range ldflagVars {
			if !seen[v] {
				t.Errorf("%s: no -X ldflag for %s", file, v)
			}
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
