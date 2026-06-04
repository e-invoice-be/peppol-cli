package backup

import (
	"net/http"

	"github.com/e-invoicebe/peppol-cli/internal/client"
)

// FingerprintForTesting exposes fingerprintAPIKey for tests in the external
// _test package. It is only compiled during `go test`.
func FingerprintForTesting(apiKey string) string {
	return fingerprintAPIKey(apiKey)
}

// PersistEnvelopeForTesting exposes persistEnvelope so security tests can
// exercise the path-validation guards without standing up a full HTTP stack.
// Only compiled during `go test`.
func PersistEnvelopeForTesting(dir string, layout Layout, httpClient *http.Client, doc client.DocumentResponse, attachments []client.DocumentAttachment) error {
	return persistEnvelope(dir, layout, httpClient, archiveEntry{
		Document:    doc,
		Attachments: attachments,
	})
}
