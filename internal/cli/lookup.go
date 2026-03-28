package cli

import (
	"errors"
	"fmt"

	"github.com/e-invoicebe/peppol-cli/internal/client"
	"github.com/e-invoicebe/peppol-cli/internal/output"
	"github.com/spf13/cobra"
)

func newLookupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "lookup [peppol-id]",
		Short:   "Look up Peppol participants",
		Long:    "Look up a Peppol participant by ID, or search participants by name.",
		Example: "  peppol lookup 0208:0123456789\n  peppol lookup search \"Company Name\" --country BE",
		Args:    cobra.MaximumNArgs(1),
		RunE:    runLookup,
	}

	cmd.AddCommand(newLookupSearchCmd())

	return cmd
}

func runLookup(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	result, err := c.LookupPeppolID(args[0])
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

	return renderLookupResult(r, result)
}

func renderLookupResult(r *output.Renderer, result *client.PeppolIdLookupResponse) error {
	pairs := []output.KVPair{
		{Key: "Status", Value: output.StatusBadge(result.Status)},
	}
	if result.QueryMetadata != nil {
		pairs = append(pairs, output.KVPair{Key: "Peppol ID", Value: result.QueryMetadata.IdentifierValue})
	}
	if result.DnsInfo != nil {
		registered := "No"
		if result.DnsInfo.Status == "success" {
			registered = "Yes"
		}
		pairs = append(pairs, output.KVPair{Key: "DNS Registered", Value: registered})
		if result.DnsInfo.SMPHostname != nil {
			pairs = append(pairs, output.KVPair{Key: "SMP Hostname", Value: *result.DnsInfo.SMPHostname})
		}
	}
	pairs = append(pairs, output.KVPair{Key: "Execution Time", Value: fmt.Sprintf("%.0fms", result.ExecutionTimeMS)})

	if err := r.KeyValue(pairs); err != nil {
		return err
	}

	// Business card entities
	if result.BusinessCard != nil && len(result.BusinessCard.Entities) > 0 {
		fmt.Fprintln(r.Writer())
		var entityPairs []output.KVPair
		for _, e := range result.BusinessCard.Entities {
			if e.Name != nil {
				entityPairs = append(entityPairs, output.KVPair{Key: "Entity Name", Value: *e.Name})
			}
			if e.CountryCode != nil {
				entityPairs = append(entityPairs, output.KVPair{Key: "Country", Value: *e.CountryCode})
			}
			for _, id := range e.Identifiers {
				entityPairs = append(entityPairs, output.KVPair{Key: "Identifier", Value: fmt.Sprintf("%s: %s", id.Scheme, id.Value)})
			}
		}
		if len(entityPairs) > 0 {
			if err := r.KeyValue(entityPairs); err != nil {
				return err
			}
		}
	}

	// Errors
	if len(result.Errors) > 0 {
		fmt.Fprintln(r.Writer())
		for _, e := range result.Errors {
			r.Error(e)
		}
	}

	return nil
}

func newLookupSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "search <query>",
		Short:   "Search Peppol participants by name",
		Example: "  peppol lookup search \"Company Name\"\n  peppol lookup search \"Company\" --country BE",
		Args:    cobra.ExactArgs(1),
		RunE:    runLookupSearch,
	}

	cmd.Flags().String("country", "", "Filter by country code (e.g. BE, NL)")

	return cmd
}

func runLookupSearch(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	country, _ := cmd.Flags().GetString("country")

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	result, err := c.SearchPeppolParticipants(args[0], country)
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

	return renderSearchResults(r, result)
}

func renderSearchResults(r *output.Renderer, result *client.PeppolSearchResult) error {
	fmt.Fprintf(r.Writer(), "Found %d participants\n\n", result.TotalCount)

	if len(result.Participants) == 0 {
		return nil
	}

	headers := []string{"PEPPOL ID", "NAME", "COUNTRY", "DOC TYPES"}
	var rows [][]string
	for _, p := range result.Participants {
		name := "-"
		country := "-"
		if len(p.Entities) > 0 {
			if p.Entities[0].Name != nil {
				name = *p.Entities[0].Name
			}
			if p.Entities[0].CountryCode != nil {
				country = *p.Entities[0].CountryCode
			}
		}
		docTypes := fmt.Sprintf("%d", len(p.DocumentTypes))
		rows = append(rows, []string{p.PeppolID, name, country, docTypes})
	}

	return r.Table(headers, rows)
}
