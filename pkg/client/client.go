package client

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform/terraform"
)

// Client is a TerraDB client
type Client struct {
	httpClient *http.Client
	URL        string
}

// NewClient returns a new TerraDB client from its URL
func NewClient(url string) *Client {
	return &Client{&http.Client{}, url}
}

// GetState returns a TerraDB state from its name and serial
// Use 0 as serial to return the latest version of the state
func (c *Client) GetState(name string, serial int) (st terraform.State, err error) {
	params := map[string]string{
		"serial": fmt.Sprintf("%v", serial),
	}

	err = c.get(&st, "states/"+name, params)
	if err != nil {
		return st, fmt.Errorf("failed to retrieve state: %v", err)
	}

	return
}

func (c *Client) get(v interface{}, path string, params map[string]string) error {
	req, err := http.NewRequest("GET", c.URL+"/"+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create http request: %v", err)
	}

	if params != nil && len(params) > 0 {
		q := req.URL.Query()

		for k, v := range params {
			q.Add(k, v)
		}

		req.URL.RawQuery = q.Encode()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform http request: %v", err)
	}

	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&v)

	return err
}
