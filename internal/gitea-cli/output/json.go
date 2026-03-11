package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// PrintJSON marshals v to indented JSON and prints to stdout.
func PrintJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	_, err = fmt.Fprintln(os.Stdout, string(data))
	return err
}
