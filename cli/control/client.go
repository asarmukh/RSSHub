package control

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Client struct{ addr string }

func NewClient(addr string) *Client { return &Client{addr: addr} }

func (c *Client) SetInterval(d time.Duration) (time.Duration, error) {
	body, _ := json.Marshal(map[string]interface{}{"duration": d.String()})
	resp, err := http.Post("http://"+c.addr+"/set-interval", "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("server error: %s", resp.Status)
	}
	var r struct {
		Old string `json:"old"`
		New string `json:"new"`
	}

	_ = json.NewDecoder(resp.Body).Decode(&r)

	log.Printf("response fields: %s, %s", r.Old, r.New)

	if r.Old != "" {
		if old, err := time.ParseDuration(r.Old); err == nil {
			return old, nil
		}
	}
	return 0, nil
}

func (c *Client) SetWorkers(n int) (int, error) {
	body, _ := json.Marshal(map[string]interface{}{"workers": n})
	resp, err := http.Post("http://"+c.addr+"/set-workers", "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("server error: %s", resp.Status)
	}
	var r struct {
		Old int `json:"old"`
		New int `json:"new"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&r)
	return r.Old, nil
}
