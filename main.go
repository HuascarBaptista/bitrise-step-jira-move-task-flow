package main

import (
	"encoding/base64"
	"fmt"
	"github.com/HuascarBaptista/bitrise-step-jira-move-task-flow/jira"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-steputils/stepconf"
	"os"
	"strings"
)

// Config ...
type Config struct {
	UserName     string `env:"user_name,required"`
	APIToken     string `env:"api_token,required"`
	BaseURL      string `env:"base_url,required"`
	IssueKeys    string `env:"jira_tickets,required"`
	TransitionId string `env:"transition,required"`
	AssigneeName string `env:"assigneeName"`
}

func main() {
	var cfg Config
	if err := stepconf.Parse(&cfg); err != nil {
		failf("Issue with input: %s", err)
	}

	stepconf.Print(cfg)
	fmt.Println()

	encodedToken := generateBase64APIToken(cfg.UserName, cfg.APIToken)
	client := jira.NewClient(encodedToken, cfg.BaseURL)
	issueKeys := strings.Split(cfg.IssueKeys, `|`)
	assigneesNames := strings.Split(cfg.AssigneeName, `|`)
	transitionIds := strings.Split(cfg.TransitionId, `|`)

	var assignees []jira.Assignee
	for index := 0; index < len(issueKeys); index++ {
		assigneeName := getCorrectValueOrEmpty(index, assigneesNames)
		transitionId := getCorrectValueOrEmpty(index, transitionIds)
		assignees = append(assignees, jira.Assignee{IssueKey: issueKeys[index], AssigneeName: assigneeName, TransitionId: transitionId})
	}

	if err := client.ChangeStatusAndAssignee(assignees); err != nil {
		failf("Posting transitions failed with error: %s", err)
	}
}

func getCorrectValueOrEmpty(i int, values []string) string {
	length := len(values)
	if length > i {
		return values[i]
	} else {
		if (length > 0) {
			return values[length-1]
		} else {
			return ""
		}
	}
}

func generateBase64APIToken(userName string, apiToken string) string {
	v := userName + `:` + apiToken
	return base64.StdEncoding.EncodeToString([]byte(v))
}

func failf(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}
