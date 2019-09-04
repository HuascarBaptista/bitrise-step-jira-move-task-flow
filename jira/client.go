package jira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/urlutil"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
)

const (
	apiEndPoint        = "/rest/api/2/issue/"
	transitionEndPoint = "/transitions?expand=transitions.fields"
)

// Client ...
type Client struct {
	token   string
	client  *http.Client
	headers map[string]string
	baseURL string
}

type Assignee struct {
	IssueKey      string
	AssigneeName  string
	TransitionIds []string
}

type response struct {
	issueKey string
	err      error
}

func (resp response) String() string {
	respValue := map[bool]string{true: colorstring.Green("SUCCES"), false: colorstring.Red("FAILED")}[resp.err == nil]
	return fmt.Sprintf("Posting change of status to - %s - : %s", resp.issueKey, respValue)
}

// -------------------------------------
// -- Public methods

// NewClient ...
func NewClient(token, baseURL string) *Client {
	return &Client{
		token:  token,
		client: &http.Client{},
		headers: map[string]string{
			"Authorization": `Basic ` + token,
			"Content-Type":  "application/json",
		},
		baseURL: baseURL,
	}
}

// ChangeStatusAndAssignee ...
func (client *Client) ChangeStatusAndAssignee(assignees []Assignee) error {
	if len(assignees) == 0 {
		return fmt.Errorf("no assignees has been added")
	}

	ch := make(chan response, len(assignees))
	for _, assignee := range assignees {
		go client.changeStatusAndAssignee(assignee, ch)
	}

	counter := 0
	var respErrors []response
	for resp := range ch {
		counter++
		log.Printf(resp.String())

		if resp.err != nil {
			respErrors = append(respErrors, resp)
		}

		if counter >= len(assignees) {
			break
		}
	}

	if len(respErrors) > 0 {
		fmt.Println()
		log.Infof("Errors during posting change of status:")

		for _, respErr := range respErrors {
			log.Warnf("Error during posting change of status to - %s - : %s", respErr.issueKey, respErr.err.Error())
		}

		fmt.Println()
	}

	return map[bool]error{true: fmt.Errorf("some change status were failed to be posted at Jira")}[len(respErrors) > 0]
}

// -------------------------------------
// -- Private methods

type JsonAssignee struct {
	Name string `json:"name,omitempty"`
}
type JsonTransition struct {
	Id string `json:"id,omitempty"`
}
type TransitionRequest struct {
	Assignee   JsonAssignee   `json:"assignee,omitempty"`
	Transition JsonTransition `json:"transition,omitempty"`
}

func (client *Client) changeStatusAndAssignee(assignee Assignee, ch chan response) {
	issueKey := assignee.IssueKey
	assigneeName := assignee.AssigneeName
	transitionIds := assignee.TransitionIds

	if len(transitionIds) == 0 {
		ch <- response{issueKey, fmt.Errorf("no transition IDs has been added")}
	}
	var errorChangingTransition error = nil
	for _, transitionId := range transitionIds {
		requestURL, err := urlutil.Join(client.baseURL, apiEndPoint, issueKey, transitionEndPoint)
		if err != nil {
			ch <- response{issueKey, err}
			return
		}
		newFields := &TransitionRequest{}
		if assigneeName != "" {
			newFields.Assignee = JsonAssignee{assigneeName}
		}
		if transitionId != "" {
			newFields.Transition = JsonTransition{transitionId}
		}
		request, err := createRequest(http.MethodPost, requestURL, client.headers, newFields)
		if err != nil {
			ch <- response{issueKey, err}
			return
		}

		requestBytes, err := httputil.DumpRequest(request, true)
		if err != nil {
			ch <- response{issueKey, err}
			return
		}
		log.Debugf("Request: %v", string(requestBytes))

		// Perform request
		_, body, err := client.performRequest(request, nil)
		log.Debugf("Body: %s", string(body))
		if (err != nil) {
			errorChangingTransition = err
		} else {
			ch <- response{issueKey, err}
			break
		}
	}
	if errorChangingTransition != nil {
		ch <- response{issueKey, errorChangingTransition}
	}

}

func createRequest(requestMethod string, url string, headers map[string]string, fields *TransitionRequest) (*http.Request, error) {
	var jsonContent []byte

	var err error
	if jsonContent, err = json.Marshal(fields); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(requestMethod, url, bytes.NewBuffer(jsonContent))
	if err != nil {
		return nil, err
	}

	addHeaders(req, headers)
	return req, nil
}

func (client *Client) performRequest(req *http.Request, requestResponse interface{}) (interface{}, []byte, error) {
	response, err := client.client.Do(req)
	if err != nil {
		// On error, any Response can be ignored
		return nil, nil, fmt.Errorf("failed to perform request, error: %s", err)
	}

	// The client must close the response body when finished with it
	defer func() {
		if cerr := response.Body.Close(); cerr != nil {
			log.Warnf("Failed to close response body, error: %s", cerr)
		}
	}()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body, error: %s", err)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode > http.StatusMultipleChoices {
		return nil, nil, fmt.Errorf("Response status: %d - Body: %s", response.StatusCode, string(body))
	}

	// Parse JSON body
	if requestResponse != nil {
		if err := json.Unmarshal([]byte(body), &requestResponse); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal response (%s), error: %s", body, err)
		}

		logDebugPretty(&requestResponse)
	}
	return requestResponse, body, nil
}

func addHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		req.Header.Set(key, value)
	}
}

func logDebugPretty(v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println("error:", err)
	}

	log.Debugf("Response: %+v\n", string(b))
}
