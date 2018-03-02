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
	demoPageURL     = "https://raw.githubusercontent.com/GenesisKernel/apps/master/demo_apps.json"
	serveKeysPort   = 85

	walletBalance = 100
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
		fmt.Println("can't get current directory: ", err)
		return
	}

	centrifugoURL = fmt.Sprintf("http://localhost:%d", centrifugoPort)
}

func main() {
	defer stopNodes()
	go waitSignal()

	start := flag.Int("start", 0, `"start = X" starts new instances in quantity of X. Previous installation data will be deleted if exists.`)
	stop := flag.Bool("stop", false, "stops current session without cleaning the data.")
	restart := flag.Bool("restart", false, "restarts previous installation.")
	clear := flag.Bool("clean", false, "removes all of the nodes and database.")
	slowInstall := flag.Bool("slow", false, "slower installation for non top machines.")
	flag.Parse()

	if (*start != 0 && (*stop || *restart || *clear)) ||
		(*stop && (*start != 0 || *restart || *clear)) ||
		(*restart && (*start != 0 || *stop || *clear)) ||
		(*clear && (*start != 0 || *stop || *restart)) {
		fmt.Println("Only one command can be used at one time.")
		return
	}

	if *start != 0 {
		startNodes(*start, 5430, *slowInstall)
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
		fmt.Println(`1. print "i" to do new installation
2. print "c" to remove all of the nodes and database
3. print "r" to restart previous installation
4. print "s" to stop services without cleaning the data
5. print "si" to slow install in case of errors during common install
6. print "q" for exit.`)
		fmt.Print("> ")
		var action string
		_, err := fmt.Scanf("%s \n", &action)
		if err != nil {
			fmt.Println("Error: ", err)
		} else {
			switch action {
			case "i":
				var nodesNumber int
				fmt.Println("print number of nodes to install. For example 3")
				fmt.Print("> ")
				_, err := fmt.Scanf("%d \n", &nodesNumber)
				if err != nil {
					fmt.Println("Error: ", err)
				} else {
					startNodes(nodesNumber, 5430, false)
				}
			case "c":
				clearNodes()
			case "r":
				restartNodes()
			case "s":
				stopNodes()
			case "si":
				var nodesNumber int
				fmt.Println("print number of nodes to install. For example 3")
				fmt.Print("> ")
				_, err := fmt.Scanf("%d \n", &nodesNumber)
				if err != nil {
					fmt.Println("Error: ", err)
				} else {
					startNodes(nodesNumber, 5430, true)
				}
			}
		}
		if action == "q" {
			break
		}
	}
}
