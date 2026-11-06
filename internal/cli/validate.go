package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/e-invoice-be/peppol-cli/internal/client"
	"github.com/e-invoice-be/peppol-cli/internal/output"
)

func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "validate",
		Short:   "Validate Peppol resources",
		Example: "  peppol validate peppol-id 0208:0123456789\n  peppol validate json invoice.json",
	}

	cmd.AddCommand(newValidatePeppolIDCmd())
	cmd.AddCommand(newValidateJSONCmd())
	cmd.AddCommand(newValidateUBLCmd())

	return cmd
}

func newValidatePeppolIDCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "peppol-id <id>",
		Short:   "Validate a Peppol participant ID",
		Long:    "Validate a Peppol ID format and check registration status in the Peppol network.",
		Example: "  peppol validate peppol-id 0208:0123456789",
		Args:    cobra.ExactArgs(1),
		RunE:    runValidatePeppolID,
	}
}

func runValidatePeppolID(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	result, err := c.ValidatePeppolID(args[0])
	if err != nil {
		if errors.Is(err, client.ErrUnauthorized) {
			return &ExitError{Err: errors.New("authentication failed (invalid API key)"), Code: 2}
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

// --- File validation commands ---

func newValidateJSONCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "json <file>",
		Short:   "Validate a JSON document against Peppol BIS Billing 3.0",
		Long:    "Validate a JSON invoice file against Peppol BIS Billing 3.0 rules.\nUse --file - to read from stdin.",
		Example: "  peppol validate json invoice.json\n  cat invoice.json | peppol validate json --file -",
		Args:    cobra.MaximumNArgs(1),
		RunE:    runValidateJSON,
	}
	cmd.Flags().String("file", "", "Read from file path (use - for stdin)")
	return cmd
}

func runValidateJSON(cmd *cobra.Command, args []string) error {
	fileFlag, _ := cmd.Flags().GetString("file")

	// Check the input before credentials, so a usage mistake is reported as
	// such (exit code 1) rather than as an auth error.
	filePath := fileFlag
	switch {
	case fileFlag == "-":
	case fileFlag == "" && len(args) == 1:
		filePath = args[0]
	case fileFlag == "":
		return &ExitError{Err: errors.New("provide a file path as argument or use --file -"), Code: 1}
	}
	if filePath != "-" {
		if _, statErr := os.Stat(filePath); statErr != nil {
			return &ExitError{Err: fmt.Errorf("file not found: %s", filePath), Code: 1}
		}
	}

	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())

	var val *client.ValidationResponse
	if filePath == "-" {
		val, err = c.ValidateJSONReader(cmd.InOrStdin())
	} else {
		val, err = c.ValidateJSON(filePath)
	}
	if err != nil {
		return handleValidateError(err)
	}

	return outputFileValidation(cmd, val)
}

func newValidateUBLCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ubl <file>",
		Short:   "Validate a UBL/XML file against Peppol BIS Billing 3.0",
		Long:    "Validate a UBL/XML invoice file against Peppol BIS Billing 3.0 rules.",
		Example: "  peppol validate ubl invoice.xml",
		Args:    cobra.ExactArgs(1),
		RunE:    runValidateUBL,
	}
}

func runValidateUBL(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	if _, err := os.Stat(filePath); err != nil {
		return &ExitError{Err: fmt.Errorf("file not found: %s", filePath), Code: 1}
	}

	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	val, err := c.ValidateUBL(filePath)
	if err != nil {
		return handleValidateError(err)
	}

	return outputFileValidation(cmd, val)
}

func outputFileValidation(cmd *cobra.Command, val *client.ValidationResponse) error {
	r := output.FromContext(cmd.Context())

	if r.IsJSON() {
		return r.JSON(val)
	}

	if err := renderFileValidation(r, val); err != nil {
		return err
	}

	if !val.IsValid {
		return &ExitError{Err: errors.New("validation failed"), Code: 3}
	}
	return nil
}

func renderFileValidation(r *output.Renderer, val *client.ValidationResponse) error {
	var errorCount, warningCount int
	for _, issue := range val.Issues {
		switch issue.Type {
		case client.IssueTypeError:
			errorCount++
		case client.IssueTypeWarning:
			warningCount++
		}
	}

	if val.IsValid {
		r.Success("Validation: PASSED")
	} else {
		r.Error(fmt.Sprintf("Validation: FAILED (%d errors, %d warnings)", errorCount, warningCount))
	}

	if len(val.Issues) == 0 {
		return nil
	}

	fmt.Fprintln(r.Writer())
	headers := []string{"SEVERITY", "RULE ID", "MESSAGE", "LOCATION"}
	var rows [][]string
	for _, issue := range val.Issues {
		severity := strings.ToUpper(string(issue.Type))
		ruleID := deref(issue.RuleID, "-")
		location := deref(issue.Location, "-")
		rows = append(rows, []string{severity, ruleID, issue.Message, location})
	}
	return r.Table(headers, rows)
}

func handleValidateError(err error) error {
	if errors.Is(err, client.ErrUnauthorized) {
		return &ExitError{Err: errors.New("authentication failed (invalid API key)"), Code: 2}
	}
	return &ExitError{Err: err, Code: 1}
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
