package main

import (
	"fmt"
	"os"
)

func clearNodes() error {
	fmt.Print("Killing processes... ")
	for _, p := range nodeProcesses {
		p.Kill()
	}
	if centrifugoProcess != nil {
		centrifugoProcess.Kill()
	}
	fmt.Println("OK")

	fmt.Print("Trying to stop postgres... ")
	err := stopPostgres()
	if err != nil {
		fmt.Println("Can't stop postgres. It seems like it already stopped.")
	} else {
		fmt.Println("OK")
	}

	fmt.Print("Trying to remove data... ")
	err = os.RemoveAll(executablePath + `\data`)
	if err != nil {
		fmt.Println(fmt.Errorf("Can't remove data directory content. Error: %s", err))
		return fmt.Errorf("Can't remove data directory content. Error: %s", err)
	}
	fmt.Println("OK")
	return nil
}
