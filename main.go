package main

import (
	"flag"
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
	demoPageURL     = "https://raw.githubusercontent.com/GenesisKernel/apps/7af853ac247315cd67fb685694ffad54bc3c4732/quick-start-simple/quick-start.json"

	walletBalance = 100

	apiBaseURL = "http://localhost:7079/api/v2"
)

var (
	executablePath    string
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

	centrifugoURL = fmt.Sprintf("http://localhost:%d", centrifugoPort)
}

func main() {
	defer stopNodes()
	go waitSignal()

	start := flag.Int("start", 0, `The "start = X" command launches X number of nodes. Previous installation data will be deleted.`)
	stop := flag.Bool("stop", false, "Stops the current session without clearing the data.")
	restart := flag.Bool("restart", false, "Restarts the previous installation attempt.")
	clear := flag.Bool("clean", false, "Removes all nodes and databases.")
	flag.Parse()

	if (*start != 0 && (*stop || *restart || *clear)) ||
		(*stop && (*start != 0 || *restart || *clear)) ||
		(*restart && (*start != 0 || *stop || *clear)) ||
		(*clear && (*start != 0 || *stop || *restart)) {
		fmt.Println("Only one command can be used at a time.")
		return
	}

	if *start != 0 {
		startNodes(*start)
		return
	}

	if *stop {
		stopNodes()
		return
	}

	if *clear {
		clearNodes()
		return
	}

	if *restart {
		restartNodes()
		return
	}

	for {
		fmt.Println(`1. Type "i" to perform a new installation 
2. Type "c" to remove all nodes and databases 
3. Type "r" to restart the previous installation 
4. Type "s" to stop the services without clearing the data 
5. Type "q" to exit.`)
		fmt.Print("> ")
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
