package backup

// FingerprintForTesting exposes fingerprintAPIKey for tests in the external
// _test package. It is only compiled during `go test`.
func FingerprintForTesting(apiKey string) string {
	return fingerprintAPIKey(apiKey)
}
