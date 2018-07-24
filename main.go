package main

import (
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/jinzhu/gorm/dialects/postgres"
)

const (
	binaryBackName  = "go-genesis.exe"
	binaryFrontName = "Genesis.exe"

	dbHost        = "localhost"
	dbPort        = 5430
	dbUser        = "postgres"
	dbPassword    = "postgres"
	dbNamePattern = "genesis%d"

	centrifugoSecret = "secret"
	centrifugoPort   = 8000

	nodeLogLevel   = "INFO"
	firstBlockFile = "1block"
	systemPort     = 7078

	waitTablesCount = 32
	demoPageURL     = "https://raw.githubusercontent.com/GenesisKernel/apps/60ddda1608e4770aabb8732127ee5c4db3facdc1/quick-start/quick-start.json"
	maxImportTx     = 10

	walletBalance = 100

	apiBaseURL = "http://localhost:7079/api/v2"
)

var (
	executablePath    string
	dataPath          string
	centrifugoURL     string
	centrifugoProcess *os.Process
	nodeProcesses     []*os.Process
	frontProcesses    []*os.Process
)

func init() {
	var err error
	executablePath, err = filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		fmt.Println("Can't find the current directory: ", err)
		return
	}
	dataPath = filepath.Join(executablePath, "data")
}

func printMenu() {
	lines := []string{
		`Type "i" to perform a new installation`,
		`Type "c" to remove all nodes and databases`,
	}

	if isInstalled() {
		lines = append(lines, `Type "r" to restart the previous installation`)
	}

	lines = append(lines,
		`Type "s" to stop the services without clearing the data`,
		`Type "q" to exit.`,
	)

	for i, line := range lines {
		fmt.Printf("%d. %s\n", i+1, line)
	}
	fmt.Print("> ")
}

func main() {
	defer stopNodes()
	go waitSignal()

	for {
		printMenu()

		var action string
		_, err := fmt.Scanf("%s \n", &action)
		if err != nil {
			fmt.Println("Error: ", err)
		} else {
			switch action {
			case "i":
				var nodesNumber int
				fmt.Println("How many nodes do you want to install? Example: 3")
				fmt.Print("> ")
				_, err := fmt.Scanf("%d \n", &nodesNumber)
				if err != nil {
					fmt.Println("Error: ", err)
				} else {
					startNodes(nodesNumber)
				}
			case "c":
				clearNodes()
			case "r":
				restartNodes()
			case "s":
				stopNodes()
			}
		}
		if action == "q" {
			break
		}
	}
}
