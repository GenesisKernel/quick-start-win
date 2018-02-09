package main

import "fmt"

type getUIDResult struct {
	UID         string `json:"uid,omitempty"`
	Token       string `json:"token,omitempty"`
	Expire      string `json:"expire,omitempty"`
	EcosystemID string `json:"ecosystem_id,omitempty"`
	KeyID       string `json:"key_id,omitempty"`
	Address     string `json:"address,omitempty"`
}

type signTestResult struct {
	Signature string `json:"signature"`
	Public    string `json:"pubkey"`
}

type loginResult struct {
	Token       string `json:"token,omitempty"`
	Refresh     string `json:"refresh,omitempty"`
	EcosystemID string `json:"ecosystem_id,omitempty"`
	KeyID       string `json:"key_id,omitempty"`
	Address     string `json:"address,omitempty"`
	NotifyKey   string `json:"notify_key,omitempty"`
	IsNode      bool   `json:"isnode,omitempty"`
	IsOwner     bool   `json:"isowner,omitempty"`
	IsVDE       bool   `json:"vde,omitempty"`
}

type prepareResult struct {
	ForSign string            `json:"forsign"`
	Signs   []TxSignJSON      `json:"signs"`
	Values  map[string]string `json:"values"`
	Time    string            `json:"time"`
}

type TxSignJSON struct {
	ForSign string    `json:"forsign"`
	Field   string    `json:"field"`
	Title   string    `json:"title"`
	Params  []SignRes `json:"params"`
}

type SignRes struct {
	Param string `json:"name"`
	Text  string `json:"text"`
}

type contractResult struct {
	Hash    string         `json:"hash"`
	Message *txstatusError `json:"errmsg,omitempty"`
	Result  string         `json:"result,omitempty"`
}

type txstatusResult struct {
	BlockID string         `json:"blockid"`
	Message *txstatusError `json:"errmsg,omitempty"`
	Result  string         `json:"result"`
}

type txstatusError struct {
	Type  string `json:"type,omitempty"`
	Error string `json:"error,omitempty"`
}

type nodeValue struct {
	Host   string
	KeyID  string
	PubKey string
}

func (nv *nodeValue) String() string {
	return fmt.Sprintf(`["%s", "%s", "%s"]`, nv.Host, nv.KeyID, nv.PubKey)
}
