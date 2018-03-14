package main

import (
	"fmt"
	"os"
)

func clearNodes() error {
	fmt.Print("Stopping the processes... ")
	for _, p := range nodeProcesses {
		p.Kill()
	}
	if centrifugoProcess != nil {
		centrifugoProcess.Kill()
	}
	fmt.Println("OK")

	fmt.Print("Stopping PostgreSQL... ")
	err := stopPostgres()
	if err != nil {
		fmt.Println("Can't stop PostgreSQL: already stopped.")
	} else {
		fmt.Println("OK")
	}

	fmt.Print("Removing data... ")
	err = os.RemoveAll(executablePath + `\data`)
	if err != nil {
		fmt.Println(fmt.Errorf("Can't remove files from the data directory. Error: %s", err))
		return fmt.Errorf("Can't remove files from the data directory. Error: %s", err)
	}
	fmt.Println("OK")
	return nil
}
