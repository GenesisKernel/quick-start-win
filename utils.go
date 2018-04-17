package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/otiai10/copy"
)

const checkPortTimeout = time.Second

var conn *sql.DB

func installPostgres() error {
	pgbinPath := executablePath + `\pgsql\bin\initdb.exe`
	c := exec.Command(pgbinPath, "-D", executablePath+`\data\pgdata`, "-U", "postgres", "-A", "trust")
	return c.Run()
}

func changePostgresPort() error {
	pgConfPath := executablePath + `\data\pgdata\postgresql.conf`
	bytes, err := ioutil.ReadFile(pgConfPath)
	if err != nil {
		return err
	}
	newFile := strings.Replace(string(bytes), "#port = 5432", "port = "+strconv.FormatInt(dbPort, 10), -1)
	return ioutil.WriteFile(pgConfPath, []byte(newFile), 0755)
}

func startPostgres() error {
	pgbinPath := executablePath + `\pgsql\bin\pg_ctl.exe`
	c := exec.Command(pgbinPath, "-D", executablePath+`\data\pgdata`, "start")
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008}
	err := c.Run()
	if err != nil {
		return fmt.Errorf("Can't start postgres. Error: %s", err)
	}
	time.Sleep(time.Second)
	for i := 1; i < 6; i++ {
		fmt.Print(fmt.Sprintf("Conducting attempt #%d... ", i))
		conn, err = sql.Open("postgres", `host=localhost port=5430 user=postgres dbname=postgres sslmode=disable password=postgres"`)
		if err != nil {
			fmt.Println("Attempt failed. Next attempt in 5 seconds.")
			time.Sleep(time.Second * 5)
			continue
		}
		err = conn.Ping()
		if err != nil {
			fmt.Println("Attempt failed. Next attempt in 5 seconds.")
			time.Sleep(time.Second * 5)
			continue
		} else {
			fmt.Println("OK")
			return nil
		}
	}
	return fmt.Errorf("Can't connect to the database")
}

func stopPostgres() error {
	pgbinPath := executablePath + `\pgsql\bin\pg_ctl.exe`
	c := exec.Command(pgbinPath, "-D", executablePath+`\data\pgdata`, "stop")
	return c.Run()
}

func getDBName(number int) string {
	return fmt.Sprintf(dbNamePattern, number)
}

func createDatabases(nodesCount int) error {
	conn, err := sql.Open("postgres", `host=localhost port=5430 user=postgres dbname=postgres sslmode=disable password=postgres"`)
	if err != nil {
		return fmt.Errorf("Can't connect to the database. Error: %s", err)
	}

	for i := 1; i <= nodesCount; i++ {
		dbName := getDBName(i)
		fmt.Print(fmt.Sprintf("Trying to create database %s... ", dbName))
		query := fmt.Sprintf(`CREATE DATABASE "%s";`, dbName)
		_, err := conn.Exec(query)
		if err != nil {
			return fmt.Errorf("Can't execute query: %s. Error:%s", query, err)
		}
		fmt.Println("OK")
	}
	return nil
}

func makeDirs(nodesCount int) error {
	for i := 1; i < nodesCount+1; i++ {
		fmt.Print(fmt.Sprintf("Creating directory %d... ", i))
		path := executablePath + `\data\` + strconv.FormatInt(int64(i), 10)
		exists, err := dirExists(path)
		if err != nil {
			fmt.Println("Error: ", err)
			return fmt.Errorf("Dir %s can't be accessed", path)
		}

		if exists {
			empty, err := dirEmpty(path)
			if err != nil {
				fmt.Println("Error: ", err)
				return fmt.Errorf("Dir %s can't be accessed", path)
			}

			if !empty {
				fmt.Println("Error: ", err)
				return fmt.Errorf("Directory %s is not empty", path)
			}
		} else {
			err := os.Mkdir(path, 0755)
			if err != nil {
				fmt.Println("Error: ", err)
				return fmt.Errorf("Can't create directory %s. Error: %s", path, err)
			}
		}
		fmt.Println("OK")
	}
	fmt.Print("Creating keys directory... ")
	path := executablePath + `\data\keys`
	exists, err := dirExists(path)
	if err != nil {
		fmt.Println("Error: ", err)
		return fmt.Errorf("Dir %s can't be accessed", path)
	}

	if exists {
		empty, err := dirEmpty(path)
		if err != nil {
			fmt.Println("Error: ", err)
			return fmt.Errorf("Dir %s can't be accessed", path)
		}

		if !empty {
			fmt.Println("Error: ", err)
			return fmt.Errorf("Directory %s is not empty", path)
		}
	} else {
		err := os.Mkdir(path, 0755)
		if err != nil {
			fmt.Println("Error: ", err)
			return fmt.Errorf("Can't create directory %s. Error: %s", path, err)
		}
	}
	fmt.Println("OK")
	return nil
}

func copyNodes(nodesCount int) error {
	for i := 1; i < nodesCount+1; i++ {
		fmt.Print(fmt.Sprintf("Copying binary files for node #%d... ", i))
		path := executablePath + `\data\` + strconv.FormatInt(int64(i), 10)
		err := copy.Copy(executablePath+`\front`, path+`\front`)
		if err != nil {
			fmt.Println("Error: ", err)
			return fmt.Errorf("Copying frontend to node #%d failed", i)
		}

		err = copy.Copy(executablePath+`\back`, path+`\back`)
		if err != nil {
			fmt.Println("Error: ", err)
			return fmt.Errorf("Copying backend to node #%d failed", i)
		}
		fmt.Println("OK")
	}
	return nil
}

func dirExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

func dirEmpty(path string) (bool, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return false, err
	}

	if len(files) > 0 {
		return false, nil
	}

	return true, nil
}

type nodeArgs interface {
	Args() []string
}

type nodeConfigArgs struct {
	dataDir    string
	firstBlock string
	dbName     string
	httpPort   int
	tcpPort    int
	nodesAddr  string
}

func (a *nodeConfigArgs) Args() []string {
	args := []string{"config"}

	args = append(args, fmt.Sprintf(`--centSecret=%s`, centrifugoSecret),
		fmt.Sprintf(`--centUrl=%s`, centrifugoURL),
		fmt.Sprintf(`--dataDir=%s`, a.dataDir),
		fmt.Sprintf(`--dbHost=%s`, dbHost),
		fmt.Sprintf(`--dbPort=%d`, dbPort),
		fmt.Sprintf(`--dbUser=%s`, dbUser),
		fmt.Sprintf(`--dbPassword=%s`, dbPassword),
		fmt.Sprintf(`--dbName=%s`, a.dbName),
		fmt.Sprintf(`--firstBlock=%s`, a.firstBlock),
		fmt.Sprintf(`--httpPort=%d`, a.httpPort),
		fmt.Sprintf(`--tcpPort=%d`, a.tcpPort),
		fmt.Sprintf(`--nodesAddr=%s`, a.nodesAddr),
		fmt.Sprintf(`--logTo=%s`, "log.txt"),
		fmt.Sprintf(`--verbosity=%s`, nodeLogLevel))

	return args
}

type nodeCommandArgs struct {
	command string
	config  string
}

func (a *nodeCommandArgs) Args() []string {
	return []string{a.command, fmt.Sprintf(`--config=%s`, a.config)}
}

func nodeCommand(args nodeArgs) (*exec.Cmd, error) {
	binaryPath := path.Join(executablePath, "back", binaryBackName)

	command := exec.Command(binaryPath, args.Args()...)
	err := command.Start()
	if err != nil {
		return nil, err
	}

	return command, nil
}

func waitNodeCommand(args nodeArgs) error {
	command, err := nodeCommand(args)
	if err != nil {
		return err
	}
	return command.Wait()
}

func createLogFile(logFilePath string) (*os.File, error) {
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return nil, fmt.Errorf("Can't open log file. Error: %e", err)
	}
	return logFile, nil
}

func startNode(nodeNumber int) error {
	configPath := path.Join(getNodePath(nodeNumber), "config.toml")
	command, err := nodeCommand(&nodeCommandArgs{"start", configPath})
	if err != nil {
		return err
	}

	nodeProcesses = append(nodeProcesses, command.Process)

	return nil
}

func getNodePath(number int) string {
	return fmt.Sprintf(`%s\data\%d\back\`, executablePath, number)
}

func initNode(nodeNumber int) error {
	port := systemPort + (nodeNumber-1)*2
	dataDir := getNodePath(nodeNumber)
	configPath := path.Join(dataDir, "config.toml")
	firstBlockPath := path.Join(getNodePath(1), firstBlockFile)

	config := &nodeConfigArgs{
		dataDir:    dataDir,
		firstBlock: firstBlockPath,
		dbName:     getDBName(nodeNumber),
		tcpPort:    port,
		httpPort:   port + 1,
	}

	if nodeNumber != 1 {
		config.nodesAddr = fmt.Sprintf("%s:%d", "127.0.0.1", systemPort)
	}

	if err := waitNodeCommand(config); err != nil {
		return err
	}

	if err := waitNodeCommand(&nodeCommandArgs{"generateKeys", configPath}); err != nil {
		return err
	}

	if nodeNumber == 1 {
		if err := waitNodeCommand(&nodeCommandArgs{"generateFirstBlock", configPath}); err != nil {
			return err
		}
	}

	return waitNodeCommand(&nodeCommandArgs{"initDatabase", configPath})
}

func initNodes(nodesCount int) error {
	for i := 1; i <= nodesCount; i++ {
		fmt.Print(fmt.Sprintf("Initializing node #%d... ", i))

		if err := initNode(i); err != nil {
			fmt.Printf("Error: %s\n", err)
			return err
		}

		fmt.Println("OK")
	}

	return nil
}

func startExistingNodes(nodesCount int, isInit bool) error {
	for i := 1; i <= nodesCount; i++ {
		fmt.Printf("Starting node #%d... ", i)

		if err := startNode(i); err != nil {
			fmt.Printf("Error: %s\n", err)
			return err
		}

		time.Sleep(5 * time.Second)

		if isInit {
			if err := waitDBstatus(i); err != nil {
				fmt.Printf("Error: %e\n", err)
				return err
			}
		}

		fmt.Println("OK")
	}

	return nil
}

func startFront(nodesCount int) error {
	for i := 1; i <= nodesCount; i++ {
		fmt.Printf("Starting fronted on node #%d... ", i)

		frontDirPath := fmt.Sprintf(`%s\data\%d\front\`, executablePath, i)
		frontExePath := frontDirPath + binaryFrontName

		port := systemPort + 2*(i-1) + 1
		apiURL := fmt.Sprintf("http://localhost:%d/api/v2", port)

		started := false
		for i := 0; i < 10; i++ {
			_, err := sendRequest("GET", apiURL+"/getuid", nil, "")
			if err != nil {
				time.Sleep(time.Second * 5)
			} else {
				started = true
				break
			}
		}
		if !started {
			fmt.Println(fmt.Sprintf("Node #%d seems to have stopped. Please check log file for more information.", i))
			continue
		}

		key, err := ioutil.ReadFile(path.Join(getNodePath(i), "PrivateKey"))
		if err != nil {
			return err
		}

		logFile, err := createLogFile(frontDirPath + "log.txt")
		if err != nil {
			fmt.Println(err)
			return err
		}

		args := make([]string, 0)
		args = append(args, "",
			fmt.Sprintf(`API_URL=%s`, apiURL),
			fmt.Sprintf(`PRIVATE_KEY=%s`, string(key)))

		procAttr := new(os.ProcAttr)
		procAttr.Files = []*os.File{logFile, logFile, logFile}
		procAttr.Dir = frontDirPath

		proc, err := os.StartProcess(frontExePath, args, procAttr)
		if err != nil {
			fmt.Println("Error: ", err)
			return err
		}
		frontProcesses = append(frontProcesses, proc)
		fmt.Println("OK")
	}

	return nil
}

func upNodes(nodesCount int) error {
	killProcesses()
	return startExistingNodes(nodesCount, false)
}

func updateFullNodes(nodesCount int) error {
	var nodes []*nodeValue
	for i := 1; i <= nodesCount; i++ {
		val, err := getFullNodeValue(i)
		if err != nil {
			return err
		}
		nodes = append(nodes, val)
	}
	b, err := json.Marshal(&nodes)
	if err != nil {
		return err
	}

	return postTx("UpdateSysParam", &url.Values{
		"Name":  {"full_nodes"},
		"Value": {string(b)},
	})
}

func updateKeys(nodesCount int) error {
	balance := walletBalance * math.Pow(10, 18)
	err := postTx("NewContract", &url.Values{
		"Value": {fmt.Sprintf(`contract InsertKey {
			data {
				KeyID int
				PubicKey string
			}
			conditions {}
			action {
				DBInsert("keys", "id,pub,amount", $KeyID, $PubicKey, "%.0f")
			}
		}`, balance)},
		"Conditions": {`ContractConditions("MainCondition")`},
	})
	if err != nil {
		return err
	}

	for i := 2; i < nodesCount+1; i++ {
		keyID, err := ioutil.ReadFile(path.Join(getNodePath(i), "KeyID"))
		if err != nil {
			return fmt.Errorf("Can't read the node's keyID. Error: %s", err)
		}

		publicKey, err := ioutil.ReadFile(path.Join(getNodePath(i), "PublicKey"))
		if err != nil {
			return fmt.Errorf("Can't read the node's public key. Error: %s", err)
		}

		err = postTx("InsertKey", &url.Values{
			"KeyID":    {string(keyID)},
			"PubicKey": {string(publicKey)},
		})

	}
	return nil
}

func sendRequest(method string, url string, payload *url.Values, auth string) ([]byte, error) {
	client := &http.Client{}

	var ioform io.Reader
	if payload != nil {
		ioform = strings.NewReader(payload.Encode())
	}

	req, err := http.NewRequest(method, url, ioform)
	if err != nil {
		return nil, fmt.Errorf("Can't create request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Can't execute request ")
	}

	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Can't read response")
	}
	return bytes, nil
}

func getFullNodeValue(nodeNumber int) (*nodeValue, error) {
	keyID, err := ioutil.ReadFile(path.Join(getNodePath(nodeNumber), "KeyID"))
	if err != nil {
		return nil, fmt.Errorf("Can't read node's KeyID. Error: %s", err)
	}

	publicKey, err := ioutil.ReadFile(path.Join(getNodePath(nodeNumber), "NodePublicKey"))
	if err != nil {
		return nil, fmt.Errorf("Can't read node's public key. Error: %s", err)
	}

	port := systemPort + (nodeNumber-1)*2

	return &nodeValue{
		TCPAddr: fmt.Sprintf("127.0.0.1:%d", port),
		APIAddr: fmt.Sprintf("http://127.0.0.1:%d", port+1),
		KeyID:   string(keyID),
		PubKey:  string(publicKey),
	}, nil
}

func dirContainFiles(dirList []os.FileInfo, necessaryFiles []string) bool {
	have := ""
	for _, f := range dirList {
		have = have + f.Name() + ","
	}
	for _, f := range necessaryFiles {
		if !strings.Contains(have, f) {
			return false
		}
	}
	return true
}

func startCentrifugo() error {
	var err error
	args := []string{`--config=config.json`, `--admin`, `--insecure_admin`, `--web`}
	procAttr := new(os.ProcAttr)
	procAttr.Dir = executablePath + `\centrifugo`
	procAttr.Files = []*os.File{os.Stdout, os.Stdin, os.Stderr}
	centrifugoProcess, err = os.StartProcess(executablePath+`\centrifugo\centrifugo.exe`, args, procAttr)
	if err != nil {
		return err
	}
	return nil
}

func waitDBstatus(number int) error {
	conn, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%d user=%s dbname=%s sslmode=disable password=%s", dbHost, dbPort, dbUser, getDBName(number), dbPassword))
	if err != nil {
		return fmt.Errorf("Can't connect to DB. Error: %s", err)
	}
	defer conn.Close()

	var tablesCount int64
	for i := 0; i < 15; i++ {
		rows, err := conn.Query(`select count(*) from information_schema.tables where table_schema='public';`)
		if err != nil {
			return fmt.Errorf("Tables count failed. Error: %s", err)
		}
		for rows.Next() {
			rows.Scan(&tablesCount)
		}
		rows.Close()
		if tablesCount < waitTablesCount {
			time.Sleep(time.Second * 5)
		} else {
			return nil
			break
		}
	}

	return fmt.Errorf("Only %d of %d tables created", tablesCount, waitTablesCount)
}

func downloadFile(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func installDemoPage() error {
	data, err := downloadFile(demoPageURL)
	if err != nil {
		return fmt.Errorf("Demo applications download failed. Error: %s", err)
	}

	err = postTx("@1Import", &url.Values{"Data": {string(data)}})
	if err != nil {
		return err
	}

	err = postTx("MembersAutoreg", &url.Values{})
	if err != nil {
		return err
	}

	fmt.Println("OK")
	return nil
}

func postTx(contract string, form *url.Values) error {
	bytes, err := ioutil.ReadFile(path.Join(getNodePath(1), "PrivateKey"))
	if err != nil {
		return fmt.Errorf("Can't read the node's private key. Error: %s", err)
	}
	privateKey := string(bytes)

	res, err := sendRequest("GET", apiBaseURL+"/getuid", nil, "")
	if err != nil {
		return err
	}

	var getUIDResult getUIDResult
	err = json.Unmarshal(res, &getUIDResult)
	if err != nil {
		return fmt.Errorf("Can't parse getuid result")
	}

	values := &url.Values{"forsign": {getUIDResult.UID}, "private": {privateKey}}
	res, err = sendRequest("POST", apiBaseURL+"/signtest/", values, "")
	if err != nil {
		return err
	}

	var signTestResult signTestResult
	err = json.Unmarshal(res, &signTestResult)
	if err != nil {
		return fmt.Errorf("Can't parse first signTest result")
	}

	fullToken := "Bearer " + getUIDResult.Token
	values = &url.Values{"pubkey": {signTestResult.Public}, "signature": {signTestResult.Signature}}
	res, err = sendRequest("POST", apiBaseURL+"/login", values, fullToken)
	if err != nil {
		return err
	}

	var loginResult loginResult
	err = json.Unmarshal(res, &loginResult)
	if err != nil {
		return fmt.Errorf("Can't parse login result")
	}
	jvtToken := "Bearer " + loginResult.Token

	res, err = sendRequest("POST", apiBaseURL+"/prepare/"+contract, form, jvtToken)
	if err != nil {
		return err
	}
	var prepareResult prepareResult
	err = json.Unmarshal(res, &prepareResult)
	if err != nil {
		return fmt.Errorf("Can't parse prepare result")
	}

	values = &url.Values{"forsign": {prepareResult.ForSign}, "private": {privateKey}}
	res, err = sendRequest("POST", apiBaseURL+"/signtest", values, "")
	if err != nil {
		return err
	}

	err = json.Unmarshal(res, &signTestResult)
	if err != nil {
		return fmt.Errorf("Can't parse second signTest result")
	}

	(*form)["time"] = []string{prepareResult.Time}
	(*form)["signature"] = []string{signTestResult.Signature}
	res, err = sendRequest("POST", apiBaseURL+"/contract/"+contract, form, jvtToken)
	if err != nil {
		return err
	}

	var contractResult contractResult

	err = json.Unmarshal(res, &contractResult)
	if err != nil {
		return fmt.Errorf("Can't parse contract result")
	}

	var txstatusResult txstatusResult
	for i := 0; i < 15; i++ {
		res, err = sendRequest("GET", apiBaseURL+"/txstatus/"+contractResult.Hash, nil, jvtToken)
		if err != nil {
			return err
		}

		err = json.Unmarshal(res, &txstatusResult)
		if err != nil {
			fmt.Println("txStatus: ", txstatusResult)
			return fmt.Errorf("Can't parse txstatus result")
		}

		if len(txstatusResult.BlockID) > 0 {
			break
		} else {
			time.Sleep(time.Second * 5)
		}
	}

	if txstatusResult.BlockID == "" {
		return fmt.Errorf("Operation timeout error")
	}

	if txstatusResult.BlockID == "0" {
		return fmt.Errorf("Can't execute request")
	}

	return nil
}

func killProcesses() {
	for _, p := range frontProcesses {
		p.Kill()
	}
	frontProcesses = make([]*os.Process, 0)

	for _, p := range nodeProcesses {
		p.Kill()
	}
	nodeProcesses = make([]*os.Process, 0)
}

func waitClose() {
	fmt.Println()
	fmt.Println(`All nodes started. Type "s" to stop the services without clearing the data.`)

	isScan := true
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			fmt.Print("> ")
			if "s" == scanner.Text() {
				stopNodes()
				return
			}

			if !isScan {
				return
			}
		}
	}()

	for _, proc := range frontProcesses {
		proc.Wait()
	}

	isScan = false
}

func waitSignal() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	for _ = range ch {
		stopNodes()
		os.Exit(0)
	}
}

func checkPorts(nodesNumber int) error {
	ports := []int{dbPort, centrifugoPort}
	for i := 1; i <= nodesNumber; i++ {
		port := systemPort + (i-1)*2
		ports = append(ports, port, port+1)
	}

	var isBusy bool
	for _, port := range ports {
		fmt.Printf("Checking port %d... ", port)

		if isFreePort(port) {
			fmt.Println("OK")
			continue
		}

		fmt.Println("Busy")
		isBusy = true
	}

	if isBusy {
		return fmt.Errorf("Please free the used ports")
	}
	return nil
}

func isFreePort(port int) bool {
	conn, err := net.DialTimeout("tcp", ":"+strconv.Itoa(port), checkPortTimeout)
	if err != nil {
		return true
	}
	defer conn.Close()
	return false
}
