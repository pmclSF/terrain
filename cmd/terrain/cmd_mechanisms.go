package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/pmclSF/terrain/internal/mechanisms"
)

// runMechanismsCLI dispatches `terrain mechanisms <verb>`. The
// available verbs are:
//
//	list   list every mechanism + state + consumer rule_ids
//	show <name>  print one mechanism's full description
//
// `terrain mechanisms` (no verb) defaults to list.
func runMechanismsCLI(args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		if len(args) == 0 {
			return runMechanismsList(os.Stdout, false)
		}
		printMechanismsUsage(os.Stderr)
		return nil
	}
	verb := args[0]
	rest := args[1:]
	switch verb {
	case "list":
		jsonOut := false
		for _, a := range rest {
			if a == "--json" {
				jsonOut = true
			}
		}
		return runMechanismsList(os.Stdout, jsonOut)
	case "show":
		if len(rest) == 0 {
			return fmt.Errorf("usage: terrain mechanisms show <name>")
		}
		return runMechanismsShow(os.Stdout, rest[0])
	default:
		printMechanismsUsage(os.Stderr)
		return fmt.Errorf("unknown verb %q", verb)
	}
}

func printMechanismsUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: terrain mechanisms <verb>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Verbs:")
	fmt.Fprintln(w, "  list [--json]    list every mechanism + state + consumer rule_ids")
	fmt.Fprintln(w, "  show <name>      print one mechanism's full description")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Per-run overrides via --mechanisms.<name>=on|off|shadow (anywhere on")
	fmt.Fprintln(w, "the command line, e.g. `terrain analyze --mechanisms.foo=shadow`).")
}

// runMechanismsList prints a table of every mechanism in the embedded
// registry: name, state, consumer rule_ids, and a one-line summary.
// --json emits the data as a JSON array instead.
func runMechanismsList(w io.Writer, jsonOut bool) error {
	reg, err := mechanisms.Load()
	if err != nil {
		return fmt.Errorf("load mechanisms registry: %w", err)
	}
	all := reg.All()
	if jsonOut {
		type listEntry struct {
			Name        string   `json:"name"`
			State       string   `json:"state"`
			Tag         string   `json:"tag"`
			Description string   `json:"description"`
			Consumers   []string `json:"consumers,omitempty"`
		}
		out := make([]listEntry, 0, len(all))
		for _, m := range all {
			tag := "preview"
			if m.State == mechanisms.StateOn {
				tag = "live"
			}
			out = append(out, listEntry{
				Name:        m.Name,
				State:       m.State.String(),
				Tag:         tag,
				Description: m.Description,
				Consumers:   m.Consumers,
			})
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	previewCount := 0
	for _, m := range all {
		if m.State != mechanisms.StateOn {
			previewCount++
		}
	}
	if previewCount > 0 {
		fmt.Fprintf(w, "Note: %d of %d mechanisms are in preview (state=shadow or off) and do not change user-visible findings.\n\n", previewCount, len(all))
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSTATE\tTAG\tCONSUMERS")
	for _, m := range all {
		consumers := "—"
		if len(m.Consumers) > 0 {
			consumers = m.Consumers[0]
			if len(m.Consumers) > 1 {
				consumers = fmt.Sprintf("%s (+%d more)", consumers, len(m.Consumers)-1)
			}
		}
		tag := "preview"
		if m.State == mechanisms.StateOn {
			tag = "live"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", m.Name, m.State.String(), tag, consumers)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Override at runtime: --mechanisms.<name>=on|off|shadow")
	fmt.Fprintln(w, "Show one mechanism's full detail: terrain mechanisms show <name>")
	return nil
}

// runMechanismsShow prints the full description + every consumer for
// one mechanism. Returns an error when the name is unknown so the
// caller can surface a non-zero exit code.
func runMechanismsShow(w io.Writer, name string) error {
	reg, err := mechanisms.Load()
	if err != nil {
		return fmt.Errorf("load mechanisms registry: %w", err)
	}
	m := reg.Get(name)
	if m == nil {
		all := reg.Names()
		return fmt.Errorf("unknown mechanism %q\n\nAvailable mechanisms: %v", name, all)
	}
	tag := "preview"
	if m.State == mechanisms.StateOn {
		tag = "live"
	}
	fmt.Fprintf(w, "Mechanism: %s\n", m.Name)
	fmt.Fprintf(w, "State:     %s (%s)\n", m.State.String(), tag)
	fmt.Fprintln(w)
	if m.Description != "" {
		fmt.Fprintln(w, "Description:")
		fmt.Fprintln(w, indent(m.Description, "  "))
	}
	if len(m.Consumers) > 0 {
		fmt.Fprintln(w, "Consumer rule_ids:")
		for _, c := range m.Consumers {
			fmt.Fprintf(w, "  - %s\n", c)
		}
	}
	return nil
}

// indent prefixes every non-empty line of s with prefix. Used to render
// the multi-line YAML descriptions in `mechanisms show` output.
func indent(s, prefix string) string {
	out := ""
	start := true
	for _, r := range s {
		if start && r != '\n' {
			out += prefix
			start = false
		}
		out += string(r)
		if r == '\n' {
			start = true
		}
	}
	return out
}
