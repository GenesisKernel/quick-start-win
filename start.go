package main

import (
	"fmt"
	"time"
)

func startNodes(nodesNumber int, psqlPort int64, slowInstall bool) {
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

	if slowInstall {
		time.Sleep(time.Minute * 2)
	}

	fmt.Print("Installing a local copy of postgres... ")
	err = installPostgres()
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	fmt.Println("OK")

	if slowInstall {
		time.Sleep(time.Minute * 2)
	}

	fmt.Print("Updating postgres config... ")
	err = changePostgresPort(psqlPort)
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

	if slowInstall {
		time.Sleep(time.Minute * 2)
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

	if slowInstall {
		time.Sleep(time.Minute * 2)
	}

	server, err := startServingFiles(nodesNumber)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	defer server.Close()

	if slowInstall {
		time.Sleep(time.Minute * 2)
	}
	fmt.Print("Updating the full_nodes parameter... ")
	err = updateFullNodes(nodesNumber)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	if slowInstall {
		time.Sleep(time.Minute * 2)
	}
	fmt.Print("Updating keys...")
	err = updateKeys(nodesNumber)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	fmt.Println("OK")

	if slowInstall {
		time.Sleep(time.Minute * 2)
	}
	fmt.Print("Installing demo applications... ")
	err = installDemoPage()
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	if slowInstall {
		time.Sleep(time.Minute * 2)
	}
	err = connectNodes(nodesNumber)
	if err != nil {
		return
	}

	err = startFront(nodesNumber)
	if err != nil {
		return
	}

	waitClose()

	return
}
