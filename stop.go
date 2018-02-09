package main

func stopNodes() error {
	killProcesses()
	stopPostgres()

	if centrifugoProcess != nil {
		centrifugoProcess.Kill()
	}

	if conn != nil {
		conn.Close()
	}

	return nil
}
