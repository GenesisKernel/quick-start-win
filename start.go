package main

import (
	"fmt"
)

func startNodes(nodesNumber int) {
	defer killProcesses()

	fmt.Println("Checking ports for availability")
	err := checkPorts(nodesNumber)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	fmt.Print("Starting centrifugo... ")
	err = startCentrifugo()
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	fmt.Println("OK")

	fmt.Print("Installing a local copy of postgres... ")
	err = installPostgres()
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	fmt.Println("OK")

	fmt.Println("Connecting to postgres database")
	err = startPostgres()
	if err != nil {
		fmt.Println(err)
	}

	err = createDatabases(nodesNumber)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = makeDirs(nodesNumber)
	if err != nil {
		return
	}

	err = copyNodes(nodesNumber)
	if err != nil {
		return
	}

	err = initNodes(nodesNumber)
	if err != nil {
		return
	}

	if err = upNodes(nodesNumber); err != nil {
		return
	}

	fmt.Print("Updating keys... ")
	err = updateKeys(nodesNumber)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	fmt.Println("OK")

	fmt.Print("Updating the full_nodes parameter... ")
	err = updateFullNodes(nodesNumber)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	fmt.Println("OK")

	fmt.Print("Installing demo applications... ")
	err = installDemoPage()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("OK")

	err = startFront(nodesNumber)
	if err != nil {
		return
	}

	waitClose()

	return
}
