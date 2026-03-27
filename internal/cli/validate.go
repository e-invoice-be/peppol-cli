package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/e-invoicebe/peppol-cli/internal/client"
	"github.com/e-invoicebe/peppol-cli/internal/output"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate Peppol resources",
	}

	cmd.AddCommand(newValidatePeppolIDCmd())

	return cmd
}

func newValidatePeppolIDCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "peppol-id <id>",
		Short: "Validate a Peppol participant ID",
		Long:  "Validate a Peppol ID format and check registration status in the Peppol network.",
		Args:  cobra.ExactArgs(1),
		RunE:  runValidatePeppolID,
	}
}

func runValidatePeppolID(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey)
	result, err := c.ValidatePeppolID(args[0])
	if err != nil {
		if errors.Is(err, client.ErrUnauthorized) {
			return &ExitError{Err: fmt.Errorf("authentication failed (invalid API key)"), Code: 2}
		}
		return &ExitError{Err: err, Code: 1}
	}

	r := output.FromContext(cmd.Context())

	if r.IsJSON() {
		return r.JSON(result)
	}

	return renderValidationResult(r, result)
}

func renderValidationResult(r *output.Renderer, result *client.PeppolIdValidationResponse) error {
	validBadge := output.StatusBadge("INVALID")
	if result.IsValid {
		validBadge = output.StatusBadge("VALID")
	}

	boolStr := func(b bool) string {
		if b {
			return "Yes"
		}
		return "No"
	}

	pairs := []output.KVPair{
		{Key: "Valid", Value: validBadge},
		{Key: "DNS Valid", Value: boolStr(result.DNSValid)},
		{Key: "Business Card Valid", Value: boolStr(result.BusinessCardValid)},
	}

	if result.BusinessCard != nil {
		if result.BusinessCard.Name != nil {
			pairs = append(pairs, output.KVPair{Key: "Name", Value: *result.BusinessCard.Name})
		}
		if result.BusinessCard.CountryCode != nil {
			pairs = append(pairs, output.KVPair{Key: "Country", Value: *result.BusinessCard.CountryCode})
		}
		if result.BusinessCard.RegistrationDate != nil {
			pairs = append(pairs, output.KVPair{Key: "Registered", Value: *result.BusinessCard.RegistrationDate})
		}
	}

	if err := r.KeyValue(pairs); err != nil {
		return err
	}

	if len(result.SupportedDocumentTypes) > 0 {
		fmt.Fprintf(r.Writer(), "\nSupported Document Types (%d):\n", len(result.SupportedDocumentTypes))
		headers := []string{"TYPE", "PROFILE"}
		var rows [][]string
		for _, dt := range result.SupportedDocumentTypes {
			docType, profile := parseDocumentTypeURN(dt)
			rows = append(rows, []string{docType, profile})
		}
		return r.Table(headers, rows)
	}

	return nil
}

// parseDocumentTypeURN extracts a readable document type and profile from a Peppol document type URN.
// Example: "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2::Invoice##urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0::2.1"
// Returns: ("Invoice", "Peppol BIS Billing 3.0")
func parseDocumentTypeURN(urn string) (string, string) {
	// Extract document type from the "::Type##" portion
	docType := urn
	if idx := strings.Index(urn, "::"); idx != -1 {
		rest := urn[idx+2:]
		if hashIdx := strings.Index(rest, "##"); hashIdx != -1 {
			docType = rest[:hashIdx]
		}
	}

	// Extract profile: look for known patterns in the URN
	profile := ""
	switch {
	case strings.Contains(urn, "poacc:billing:"):
		profile = "Peppol BIS Billing 3.0"
	case strings.Contains(urn, "poacc:selfbilling:"):
		profile = "Peppol Self-Billing 3.0"
	case strings.Contains(urn, "nlcius"):
		profile = "NL CIUS"
	case strings.Contains(urn, "efff"):
		profile = "BE EFFF"
	default:
		// Fall back to showing the customization ID portion after ##
		if idx := strings.Index(urn, "##"); idx != -1 {
			profile = urn[idx+2:]
			// Trim the version suffix "::2.1"
			if verIdx := strings.LastIndex(profile, "::"); verIdx != -1 {
				profile = profile[:verIdx]
			}
		}
	}

	return docType, profile
}
