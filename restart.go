package main

import (
	"fmt"
	"io/ioutil"
)

func restartNodes() error {
	defer killProcesses()

	killProcesses()

	files, err := ioutil.ReadDir(executablePath + `\data`)
	if err != nil {
		return fmt.Errorf("Data directory not found")
	}
	nodesCount := len(files) - 2

	fmt.Println("Checking ports for availability...")
	err = checkPorts(nodesCount)
	if err != nil {
		fmt.Println("Error: ", err)
		return err
	}

	server, err := startServingFiles(len(files) - 2)
	if err != nil {
		return err
	}
	defer server.Close()

	fmt.Print("Starting centrifugo... ")
	err = startCentrifugo()
	if err != nil {
		fmt.Println("Error: ", err)
		return err
	}
	fmt.Println("OK")

	fmt.Println("Starting postgres... ")
	err = startPostgres()
	if err != nil {
		fmt.Printf("Can't start postgres. Error: %s", err)
		return fmt.Errorf("Can't start postgres. Error: ", err)
	}

	err = upNodes(nodesCount)
	if err != nil {
		return err
	}

	err = startFront(nodesCount)
	if err != nil {
		return err
	}

	waitClose()

	return nil
}
