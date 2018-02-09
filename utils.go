package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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

var conn *sql.DB

func installPostgres() error {
	pgbinPath := executablePath + `\pgsql\bin\initdb.exe`
	commandArgs := ` -D ` + executablePath + `\data\pgdata -U postgres -E UTF8 -A trust`
	c := exec.Command("cmd", "/C "+pgbinPath+commandArgs)
	return c.Run()
}

func changePostgresPort(port int64) error {
	conf, err := os.OpenFile(executablePath+`\data\pgdata\postgresql.conf`, os.O_RDWR, 0755)
	if err != nil {
		return err
	}

	bytes, err := ioutil.ReadAll(conf)
	if err != nil {
		return err
	}

	newFile := strings.Replace(string(bytes), "#port = 5432", "port = "+strconv.FormatInt(port, 10), -1)

	conf.Close()

	return ioutil.WriteFile(executablePath+`\data\pgdata\postgresql.conf`, []byte(newFile), 0755)
}

func startPostgres() error {
	pgbinPath := executablePath + `\pgsql\bin\pg_ctl.exe`
	commandArgs := ` -D ` + executablePath + `\data\pgdata start`
	c := exec.Command("cmd", "/C "+pgbinPath+commandArgs)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008}
	err := c.Run()
	if err != nil {
		return fmt.Errorf("Can't start postgres. Error: %s", err)
	}
	time.Sleep(time.Second)
	for i := 1; i < 6; i++ {
		fmt.Print(fmt.Sprintf("%d attempt... ", i))
		conn, err = sql.Open("postgres", `host=localhost port=5430 user=postgres dbname=postgres sslmode=disable password=postgres"`)
		if err != nil {
			fmt.Println("Fail. Wait 5 seconds before next attempt.")
			time.Sleep(time.Second * 5)
			continue
		}
		err = conn.Ping()
		if err != nil {
			fmt.Println("Fail. Wait 5 seconds before next attempt.")
			time.Sleep(time.Second * 5)
			continue
		} else {
			fmt.Println("OK")
			return nil
		}
	}
	return fmt.Errorf("Can't connect to database")
}

func stopPostgres() error {
	pgbinPath := executablePath + `\pgsql\bin\pg_ctl.exe`
	commandArgs := ` -D ` + executablePath + `\data\pgdata stop`
	c := exec.Command("cmd", "/C "+pgbinPath+commandArgs)

	return c.Run()
}

func getDBName(number int) string {
	return fmt.Sprintf(dbNamePattern, number)
}

func createDatabases(nodesCount int) error {
	conn, err := sql.Open("postgres", `host=localhost port=5430 user=postgres dbname=postgres sslmode=disable password=postgres"`)
	if err != nil {
		return fmt.Errorf("Can't connect database. Error: %s", err)
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
		fmt.Print(fmt.Sprintf("Trying to create directory %d... ", i))
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
				return fmt.Errorf("dir %s is not empty", path)
			}
		} else {
			err := os.Mkdir(path, 0755)
			if err != nil {
				fmt.Println("Error: ", err)
				return fmt.Errorf("can't create directory %s. Error: %s", path, err)
			}
		}
		fmt.Println("OK")
	}
	fmt.Print("Trying to create keys directory... ")
	path := executablePath + `\data\keys`
	exists, err := dirExists(path)
	if err != nil {
		fmt.Println("Error: ", err)
		return fmt.Errorf("dir %s can't be accessed", path)
	}

	if exists {
		empty, err := dirEmpty(path)
		if err != nil {
			fmt.Println("Error: ", err)
			return fmt.Errorf("dir %s can't be accessed", path)
		}

		if !empty {
			fmt.Println("Error: ", err)
			return fmt.Errorf("dir %s is not empty", path)
		}
	} else {
		err := os.Mkdir(path, 0755)
		if err != nil {
			fmt.Println("Error: ", err)
			return fmt.Errorf("can't create directory %s. Error: %s", path, err)
		}
	}
	fmt.Println("OK")
	return nil
}

func copyNodes(nodesCount int) error {
	for i := 1; i < nodesCount+1; i++ {
		fmt.Print(fmt.Sprintf("Copy %d node binaries... ", i))
		path := executablePath + `\data\` + strconv.FormatInt(int64(i), 10)
		err := copy.Copy(executablePath+`\front`, path+`\front`)
		if err != nil {
			fmt.Println("Error: ", err)
			return fmt.Errorf("can't copy frontend to %d node", i)
		}

		err = copy.Copy(executablePath+`\back`, path+`\back`)
		if err != nil {
			fmt.Println("Error: ", err)
			return fmt.Errorf("can't copy backend to %d node", i)
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

type nodeArgs struct {
	workDir            string
	initConfig         bool
	tcpPort            int
	httpPort           int
	initDatabase       bool
	dbName             string
	generateFirstBlock bool
	firstBlockPath     string
	noStart            bool
	keyID              int
}

func (a *nodeArgs) Args() []string {
	args := make([]string, 0)
	args = append(args, fmt.Sprintf(`-workDir=%s`, a.workDir),
		fmt.Sprintf(`-initConfig=%t`, a.initConfig),
		fmt.Sprintf(`-tcpPort=%d`, a.tcpPort),
		fmt.Sprintf(`-httpPort=%d`, a.httpPort),
		fmt.Sprintf(`-initDatabase=%t`, a.initDatabase),
		fmt.Sprintf(`-dbHost=%s`, dbHost),
		fmt.Sprintf(`-dbPort=%d`, dbPort),
		fmt.Sprintf(`-dbName=%s`, a.dbName),
		fmt.Sprintf(`-dbUser=%s`, dbUser),
		fmt.Sprintf(`-dbPassword=%s`, dbPassword),
		fmt.Sprintf(`-generateFirstBlock=%t`, a.generateFirstBlock),
		fmt.Sprintf(`-firstBlockPath=%s`, a.firstBlockPath),
		fmt.Sprintf(`-centrifugoSecret=%s`, centrifugoSecret),
		fmt.Sprintf(`-centrifugoUrl=%s`, centrifugoURL),
		fmt.Sprintf(`-keyID=%d`, a.keyID),
		fmt.Sprintf(`-privateBlockchain=%d`, 1),
		fmt.Sprintf(`-noStart=%t`, a.noStart),
		fmt.Sprintf(`-logLevel=%s`, nodeLogLevel))

	return args
}

func createLogFile(logFilePath string) (*os.File, error) {
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return nil, fmt.Errorf("Can't open log file. Error: %e", err)
	}
	return logFile, nil
}

func startNode(args *nodeArgs) error {
	binaryPath := path.Join(executablePath, "back", binaryBackName)

	logFile, err := createLogFile(path.Join(args.workDir, "log.txt"))
	if err != nil {
		return err
	}

	procAttr := &os.ProcAttr{
		Files: []*os.File{nil, logFile, logFile},
		Dir:   args.workDir,
	}

	proc, err := os.StartProcess(binaryPath, args.Args(), procAttr)
	if err != nil {
		return err
	}

	if args.noStart {
		proc.Wait()
		return nil
	}

	nodeProcesses = append(nodeProcesses, proc)

	return nil
}

func getNodePath(number int) string {
	return fmt.Sprintf(`%s\data\%d\back\`, executablePath, number)
}

func initNodes(nodesCount int) error {
	firstNodePath := getNodePath(1)
	firstBlockPath := firstNodePath + firstBlockFile

	fmt.Print("Trying to start 1 node... ")
	err := startNode(&nodeArgs{
		workDir:            firstNodePath,
		initConfig:         true,
		tcpPort:            systemPort,
		httpPort:           systemPort + 1,
		initDatabase:       true,
		dbName:             getDBName(1),
		generateFirstBlock: true,
		firstBlockPath:     firstBlockPath,
	})
	if err != nil {
		fmt.Println(err)
		return err
	}
	err = waitDBstatus(1)
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println("OK")

	for i := 2; i <= nodesCount; i++ {
		fmt.Print(fmt.Sprintf("Trying to init %d node... ", i))
		port := systemPort + (i-1)*2
		err := startNode(&nodeArgs{
			workDir:            getNodePath(i),
			initConfig:         true,
			tcpPort:            port,
			httpPort:           port + 1,
			initDatabase:       true,
			dbName:             getDBName(i),
			generateFirstBlock: true,
			firstBlockPath:     os.DevNull,
			noStart:            true,
		})
		if err != nil {
			fmt.Println(err)
			return err
		}
		fmt.Println("OK")
	}

	return nil
}

func connectNodes(nodesCount int) error {
	return startExistingNodes(2, nodesCount, true)
}

func startExistingNodes(offsetNode, nodesCount int, isInit bool) error {
	firstBlockPath := getNodePath(1) + firstBlockFile

	for i := offsetNode; i <= nodesCount; i++ {
		fmt.Print(fmt.Sprintf("Trying to start %d node... ", i))

		port := systemPort + (i-1)*2

		nodePath := getNodePath(i)
		data, err := ioutil.ReadFile(nodePath + "KeyID")
		if err != nil {
			fmt.Println(err)
			return err
		}
		keyID, err := strconv.Atoi(string(data))
		if err != nil {
			fmt.Println(err)
			return err
		}

		err = startNode(&nodeArgs{
			workDir:        nodePath,
			initConfig:     isInit,
			tcpPort:        port,
			httpPort:       port + 1,
			initDatabase:   isInit,
			dbName:         getDBName(i),
			firstBlockPath: firstBlockPath,
			keyID:          keyID,
		})
		if err != nil {
			fmt.Println(err)
			return err
		}

		time.Sleep(5 * time.Second)

		if isInit {
			err = waitDBstatus(i)
			if err != nil {
				fmt.Println(err)
				return err
			}
		}

		fmt.Println("OK")
	}

	return nil
}

func startFront(nodesCount int) error {
	for i := 1; i <= nodesCount; i++ {
		fmt.Printf("Trying to start %d fronted... ", i)

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
			fmt.Println(fmt.Sprintf("It seems like %d node stopped. Check log file for more information.", i))
			continue
		}

		logFile, err := createLogFile(frontDirPath + "log.txt")
		if err != nil {
			fmt.Println(err)
			return err
		}

		args := make([]string, 0)
		args = append(args, "", fmt.Sprintf(`API_URL=%s`, apiURL),
			fmt.Sprintf(`PRIVATE_KEY=http://localhost:85/%d`, i))

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
	return startExistingNodes(1, nodesCount, false)
}

func startServingFiles(nodesCount int) (*http.Server, error) {
	keysDir := executablePath + `\data\keys\`
	nodeKeyPath := executablePath + `\data\%d\back\PrivateKey`

	server := &http.Server{Addr: "localhost:85"}

	for i := 1; i < nodesCount+1; i++ {
		err := copy.Copy(fmt.Sprintf(nodeKeyPath, i), keysDir+fmt.Sprintf("%d", i))
		if err != nil {
			return nil, err
		}
	}
	go func() {
		mux := http.NewServeMux()
		fs := http.FileServer(http.Dir(keysDir))
		mux.Handle("/", fs)

		server.Handler = mux
		err := server.ListenAndServe()
		if err != nil {
			return
		}
	}()

	return server, nil
}

func updateFullNodes(nodesCount int) error {
	privateKey := ""
	for i := 0; i < 10; i++ {
		pKey, err := os.OpenFile(executablePath+`\data\1\back\PrivateKey`, os.O_RDONLY, 0755)
		if err != nil {
			time.Sleep(time.Second * 5)
			continue
		}
		bytes, err := ioutil.ReadAll(pKey)
		if err != nil {
			return fmt.Errorf("can't read node private key. Error: %s", err)
		}
		privateKey = string(bytes)
		pKey.Close()
		break
	}
	if privateKey == "" {
		return fmt.Errorf("can't open node private key")
	}
	newVals := ""
	for i := 1; i <= nodesCount; i++ {
		val, err := getFullNodeValue(i)
		if err != nil {
			return err
		}
		newVals += val.String() + ","
	}
	newVals = "[" + strings.TrimRight(newVals, ",") + "]"

	baseURL := "http://localhost:7079/api/v2"
	res, err := sendRequest("GET", baseURL+"/getuid", nil, "")
	if err != nil {
		return err
	}

	var getUIDResult getUIDResult
	err = json.Unmarshal(res, &getUIDResult)
	if err != nil {
		return fmt.Errorf("can't parse first getuid result")
	}

	values := &url.Values{"forsign": {getUIDResult.UID}, "private": {privateKey}}
	res, err = sendRequest("POST", baseURL+"/signtest/", values, "")
	if err != nil {
		return err
	}

	var signTestResult signTestResult
	err = json.Unmarshal(res, &signTestResult)
	if err != nil {
		return fmt.Errorf("can't parse first signTest result")
	}

	fullToken := "Bearer " + getUIDResult.Token
	values = &url.Values{"pubkey": {signTestResult.Public}, "signature": {signTestResult.Signature}}
	res, err = sendRequest("POST", baseURL+"/login", values, fullToken)
	if err != nil {
		return err
	}

	var loginResult loginResult
	err = json.Unmarshal(res, &loginResult)
	if err != nil {
		return fmt.Errorf("can't parse first login result")
	}
	jvtToken := "Bearer " + loginResult.Token

	values = &url.Values{"Name": {"full_nodes"}, "Value": {newVals}}
	res, err = sendRequest("POST", baseURL+"/prepare/UpdateSysParam", values, jvtToken)
	if err != nil {
		return err
	}
	var prepareResult prepareResult
	err = json.Unmarshal(res, &prepareResult)
	if err != nil {
		return fmt.Errorf("can't parse first prepare result")
	}

	values = &url.Values{"forsign": {prepareResult.ForSign}, "private": {privateKey}}
	res, err = sendRequest("POST", baseURL+"/signtest", values, "")
	if err != nil {
		return err
	}

	err = json.Unmarshal(res, &signTestResult)
	if err != nil {
		return fmt.Errorf("can't parse second signTest result")
	}

	values = &url.Values{"Name": {"full_nodes"}, "Value": {newVals}, "time": {prepareResult.Time}, "signature": {signTestResult.Signature}}
	res, err = sendRequest("POST", baseURL+"/contract/UpdateSysParam", values, jvtToken)
	if err != nil {
		return err
	}

	var contractResult contractResult

	err = json.Unmarshal(res, &contractResult)
	if err != nil {
		return fmt.Errorf("can't parse contract result")
	}

	var txstatusResult txstatusResult
	for i := 0; i < 10; i++ {
		res, err = sendRequest("GET", baseURL+"/txstatus/"+contractResult.Hash, nil, jvtToken)
		if err != nil {
			return err
		}

		err = json.Unmarshal(res, &txstatusResult)
		if err != nil {
			fmt.Println("txStatus: ", txstatusResult)
			return fmt.Errorf("can't parse txstatus result")
		}
		if txstatusResult.BlockID == "" {
			time.Sleep(time.Second * 5)
		} else {
			break
		}
	}

	if txstatusResult.BlockID == "" {
		return fmt.Errorf("timeout error")
	}

	if txstatusResult.BlockID == "0" {
		return fmt.Errorf("can't update system parameters")
	}
	fmt.Println("OK")
	return nil
}

func updateKeys(nodesCount int) error {
	baseURL := "http://localhost:7079/api/v2"

	pKey, err := os.OpenFile(executablePath+`\data\1\back\PrivateKey`, os.O_RDONLY, 0755)
	if err != nil {
		return fmt.Errorf("can't open node private key. Error: %s", err)
	}
	defer pKey.Close()
	bytes, err := ioutil.ReadAll(pKey)
	if err != nil {
		return fmt.Errorf("can't read  node private key. Error: %s", err)
	}
	privateKey := string(bytes)

	for i := 2; i < nodesCount+1; i++ {
		kID, err := os.OpenFile(executablePath+fmt.Sprintf(`\data\%d\back\KeyID`, i), os.O_RDONLY, 0755)
		if err != nil {
			return fmt.Errorf("can't open node private key. Error: %s", err)
		}
		defer kID.Close()
		bytes, err := ioutil.ReadAll(kID)
		if err != nil {
			return fmt.Errorf("can't read  node private key. Error: %s", err)
		}
		keyID := string(bytes)

		pub, err := os.OpenFile(executablePath+fmt.Sprintf(`\data\%d\back\PublicKey`, i), os.O_RDONLY, 0755)
		if err != nil {
			return fmt.Errorf("can't open node private key. Error: %s", err)
		}
		defer pub.Close()
		bytes, err = ioutil.ReadAll(pub)
		if err != nil {
			return fmt.Errorf("can't read  node private key. Error: %s", err)
		}
		pubKey := string(bytes)

		res, err := sendRequest("GET", baseURL+"/getuid", nil, "")
		if err != nil {
			return err
		}
		var getUIDResult getUIDResult
		err = json.Unmarshal(res, &getUIDResult)
		if err != nil {
			return fmt.Errorf("can't parse getuid result")
		}

		values := &url.Values{"forsign": {getUIDResult.UID}, "private": {privateKey}}
		res, err = sendRequest("POST", baseURL+"/signtest/", values, "")
		if err != nil {
			return err
		}

		var signTestResult signTestResult
		err = json.Unmarshal(res, &signTestResult)
		if err != nil {
			return fmt.Errorf("can't parse first signTest result")
		}

		fullToken := "Bearer " + getUIDResult.Token
		values = &url.Values{"pubkey": {signTestResult.Public}, "signature": {signTestResult.Signature}}
		res, err = sendRequest("POST", baseURL+"/login", values, fullToken)
		if err != nil {
			return err
		}

		var loginResult loginResult
		err = json.Unmarshal(res, &loginResult)
		if err != nil {
			return fmt.Errorf("can't parse login result")
		}

		jvtToken := "Bearer " + loginResult.Token

		contName := "con_updatekeys" + strconv.FormatInt(int64(i), 10)
		code := `{data {}conditions {} action {$result=DBInsert("keys", "id,pub,amount", "` + keyID + `", "` + pubKey + `", "100") }}`
		updateKeysCode := "contract " + contName + code
		values = &url.Values{"Wallet": {""}, "Value": {updateKeysCode}, "Conditions": {`"ContractConditions(` + "`MainCondition`" + `)"`}}

		res, err = sendRequest("POST", baseURL+"/prepare/NewContract", values, jvtToken)
		if err != nil {
			return err
		}
		var prepareResult prepareResult
		err = json.Unmarshal(res, &prepareResult)
		if err != nil {
			return fmt.Errorf("can't parse first prepare result")
		}

		values = &url.Values{"forsign": {prepareResult.ForSign}, "private": {privateKey}}
		res, err = sendRequest("POST", baseURL+"/signtest/", values, "")
		if err != nil {
			return err
		}

		err = json.Unmarshal(res, &signTestResult)
		if err != nil {
			return fmt.Errorf("can't parse first signTest result")
		}

		values = &url.Values{"Wallet": {""}, "Value": {updateKeysCode}, "Conditions": {`"ContractConditions(` + "`MainCondition`" + `)"`}, "time": {prepareResult.Time}, "signature": {signTestResult.Signature}}
		res, err = sendRequest("POST", baseURL+"/contract/NewContract", values, jvtToken)
		if err != nil {
			return err
		}

		var contractResult contractResult
		err = json.Unmarshal(res, &contractResult)
		if err != nil {
			return fmt.Errorf("can't parse first contract result")
		}

		var txstatusResult txstatusResult
		for i := 0; i < 10; i++ {
			res, err = sendRequest("GET", baseURL+"/txstatus/"+contractResult.Hash, nil, jvtToken)
			if err != nil {
				return err
			}

			err = json.Unmarshal(res, &txstatusResult)
			if err != nil {
				return fmt.Errorf("can't parse txstatus result")
			}
			if txstatusResult.BlockID == "" {
				time.Sleep(time.Second * 5)
			} else {
				break
			}
		}
		if txstatusResult.BlockID == "" {
			return fmt.Errorf("timeout 1 error")
		}

		if txstatusResult.BlockID == "0" {
			return fmt.Errorf("update keys request failed")
		}

		values = &url.Values{}
		res, err = sendRequest("POST", baseURL+"/prepare/"+contName, values, jvtToken)
		if err != nil {
			return err
		}

		err = json.Unmarshal(res, &prepareResult)
		if err != nil {
			return fmt.Errorf("can't parse second prepare result")
		}

		values = &url.Values{"forsign": {prepareResult.ForSign}, "private": {privateKey}}
		res, err = sendRequest("POST", baseURL+"/signtest/", values, "")
		if err != nil {
			return err
		}

		err = json.Unmarshal(res, &signTestResult)
		if err != nil {
			return fmt.Errorf("can't parse second signTest result")
		}

		values = &url.Values{"time": {prepareResult.Time}, "signature": {signTestResult.Signature}}
		res, err = sendRequest("POST", baseURL+"/contract/"+contName, values, jvtToken)
		if err != nil {
			return err
		}

		err = json.Unmarshal(res, &contractResult)
		if err != nil {
			return fmt.Errorf("can't parse second contract result")
		}

		for i := 0; i < 10; i++ {
			res, err = sendRequest("GET", baseURL+"/txstatus/"+contractResult.Hash, nil, jvtToken)
			if err != nil {
				return err
			}

			err = json.Unmarshal(res, &txstatusResult)
			if err != nil {
				return fmt.Errorf("can't parse txstatus result")
			}
			if txstatusResult.BlockID == "" {
				time.Sleep(time.Second * 5)
			} else {
				break
			}
		}

		if txstatusResult.BlockID == "" {
			return fmt.Errorf("timeout 2 error")
		}

		if txstatusResult.BlockID == "0" {
			return fmt.Errorf("update keys contract failed")
		}

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
		return nil, fmt.Errorf("can't create request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("can't exec request ")
	}

	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("can't read request")
	}
	return bytes, nil
}

func getFullNodeValue(nodeNumber int) (nodeValue, error) {
	keyID := ""
	publicKey := ""
	for i := 0; i < 10; i++ {
		kID, err := os.OpenFile(executablePath+`\data\`+strconv.FormatInt(int64(nodeNumber), 10)+`\back\KeyID`, os.O_RDONLY, 0755)
		if err != nil {
			time.Sleep(time.Second * 5)
			continue
		}
		defer kID.Close()
		bytes, err := ioutil.ReadAll(kID)
		if err != nil {
			return nodeValue{}, fmt.Errorf("can't read  node keyID. Error: %s", err)
		}
		keyID = string(bytes)
		break
	}

	for i := 0; i < 10; i++ {
		publicKeyFile, err := os.OpenFile(executablePath+`\data\`+strconv.FormatInt(int64(nodeNumber), 10)+`\back\NodePublicKey`, os.O_RDONLY, 0755)
		if err != nil {
			time.Sleep(time.Second * 5)
			continue
		}
		defer publicKeyFile.Close()
		publicBytes, err := ioutil.ReadAll(publicKeyFile)
		if err != nil {
			return nodeValue{}, fmt.Errorf("can't read  node public key. Error: %s", err)
		}
		publicKey = string(publicBytes)
		break
	}

	if keyID == "" {
		return nodeValue{}, fmt.Errorf("can't open node %d KeyID", nodeNumber)
	}
	if publicKey == "" {
		return nodeValue{}, fmt.Errorf("can't open node %d NodePublicKey", nodeNumber)
	}

	if nodeNumber == 1 {
		return nodeValue{Host: "127.0.0.1", KeyID: keyID, PubKey: publicKey}, nil
	}

	return nodeValue{Host: "127.0.0.1:" + strconv.FormatInt(int64(systemPort+(nodeNumber-1)*2), 10), KeyID: keyID, PubKey: publicKey}, nil
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
		return fmt.Errorf("Can't connect to db. Error: %s", err)
	}
	defer conn.Close()

	var tablesCount int64
	for i := 0; i < 15; i++ {
		rows, err := conn.Query(`select count(*) from information_schema.tables where table_schema='public';`)
		if err != nil {
			return fmt.Errorf("Can't get tables count. Error: %s", err)
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

	return fmt.Errorf("Only %d tables of %d created", tablesCount, waitTablesCount)
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
		return fmt.Errorf("Can't download demo_page application. Error: %s", err)
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
	privateKey := ""
	for i := 0; i < 10; i++ {
		pKey, err := os.OpenFile(executablePath+`\data\1\back\PrivateKey`, os.O_RDONLY, 0755)
		if err != nil {
			time.Sleep(time.Second * 5)
			continue
		}
		bytes, err := ioutil.ReadAll(pKey)
		if err != nil {
			return fmt.Errorf("can't read node private key. Error: %s", err)
		}
		privateKey = string(bytes)
		pKey.Close()
		break
	}
	if privateKey == "" {
		return fmt.Errorf("can't open node private key")
	}

	baseURL := "http://localhost:7079/api/v2"
	res, err := sendRequest("GET", baseURL+"/getuid", nil, "")
	if err != nil {
		return err
	}

	var getUIDResult getUIDResult
	err = json.Unmarshal(res, &getUIDResult)
	if err != nil {
		return fmt.Errorf("can't parse first getuid result")
	}

	values := &url.Values{"forsign": {getUIDResult.UID}, "private": {privateKey}}
	res, err = sendRequest("POST", baseURL+"/signtest/", values, "")
	if err != nil {
		return err
	}

	var signTestResult signTestResult
	err = json.Unmarshal(res, &signTestResult)
	if err != nil {
		return fmt.Errorf("can't parse first signTest result")
	}

	fullToken := "Bearer " + getUIDResult.Token
	values = &url.Values{"pubkey": {signTestResult.Public}, "signature": {signTestResult.Signature}}
	res, err = sendRequest("POST", baseURL+"/login", values, fullToken)
	if err != nil {
		return err
	}

	var loginResult loginResult
	err = json.Unmarshal(res, &loginResult)
	if err != nil {
		return fmt.Errorf("can't parse first login result")
	}
	jvtToken := "Bearer " + loginResult.Token

	res, err = sendRequest("POST", baseURL+"/prepare/"+contract, form, jvtToken)
	if err != nil {
		return err
	}
	var prepareResult prepareResult
	err = json.Unmarshal(res, &prepareResult)
	if err != nil {
		return fmt.Errorf("can't parse first prepare result")
	}

	values = &url.Values{"forsign": {prepareResult.ForSign}, "private": {privateKey}}
	res, err = sendRequest("POST", baseURL+"/signtest", values, "")
	if err != nil {
		return err
	}

	err = json.Unmarshal(res, &signTestResult)
	if err != nil {
		return fmt.Errorf("can't parse second signTest result")
	}

	(*form)["time"] = []string{prepareResult.Time}
	(*form)["signature"] = []string{signTestResult.Signature}
	res, err = sendRequest("POST", baseURL+"/contract/"+contract, form, jvtToken)
	if err != nil {
		return err
	}

	var contractResult contractResult

	err = json.Unmarshal(res, &contractResult)
	if err != nil {
		return fmt.Errorf("can't parse contract result")
	}

	var txstatusResult txstatusResult
	for i := 0; i < 15; i++ {
		res, err = sendRequest("GET", baseURL+"/txstatus/"+contractResult.Hash, nil, jvtToken)
		if err != nil {
			return err
		}

		err = json.Unmarshal(res, &txstatusResult)
		if err != nil {
			fmt.Println("txStatus: ", txstatusResult)
			return fmt.Errorf("can't parse txstatus result")
		}

		if len(txstatusResult.BlockID) > 0 {
			break
		} else {
			time.Sleep(time.Second * 5)
		}
	}

	if txstatusResult.BlockID == "" {
		return fmt.Errorf("timeout error")
	}

	if txstatusResult.BlockID == "0" {
		return fmt.Errorf("can't update system parameters")
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
	fmt.Println(`All nodes started. Print "s" to stop services without cleaning the data.`)

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
