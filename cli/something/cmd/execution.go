package cmd

import "fmt"

func requireExecutionConfirmation(confirmed bool) error {
	if confirmed {
		return nil
	}
	return fmt.Errorf("pass --confirm to allow launching saved desktop applications")
}
