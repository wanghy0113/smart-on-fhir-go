package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"text/template"
)

var smartConfig *SmartConfig

type SmartClient struct {
	endPoint    string
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	Intent      string `json:"intent"`
	Encounter   string `json:"encounter"`
}

func (c *SmartClient) getEncounter() string {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", c.endPoint+"/Encounter?_id="+c.Encounter, nil)
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	resp, _ := client.Do(req)
	b, _ := ioutil.ReadAll(resp.Body)
	return string(b)
}

type SmartConfig struct {
	EHREndpoint           string
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	ScopesSupported       []string `json:"scopes_supported"`
	Capabilities          []string `json:"capabilities"`
}

func (c *SmartConfig) NewSmartClient(code string) (*SmartClient, error) {
	params := &struct {
		ClientID    string
		RedirectURI string
		Code        string
	}{"test_client", "http://localhost:8080/authenticate", code}
	tml, _ := template.New("body").Parse("grant_type=authorization_code&client_id={{.ClientID}}&code={{.Code}}&redirect_uri={{.RedirectURI}}")
	body := &bytes.Buffer{}
	tml.Execute(body, params)
	resp, err := http.Post(c.TokenEndpoint, "application/x-www-form-urlencoded", body)
	if err != nil {
		log.Printf("Getting access token err: %v", err)
		return nil, err
	}
	//b, _ := ioutil.ReadAll(resp.Body)
	//log.Printf("Get access token resp: %v", string(b))
	client := &SmartClient{}
	err = json.NewDecoder(resp.Body).Decode(client)
	if err != nil {
		log.Printf("json.NewDecoder(resp.Body).Decode(client) err: %v", err)
		return nil, err
	}
	client.endPoint = c.EHREndpoint
	log.Printf("Created smart client: %v", client)
	return client, nil
}

func (c *SmartConfig) authorizationURL(iss, launchID string) string {
	params := &struct {
		EndPoint    string
		ClientID    string
		RedirectURI string
		Launch      string
		Scope       string
		State       string
		Aud         string
	}{c.AuthorizationEndpoint, "test_client", "http://localhost:8080/authenticate", launchID, "openid fhirUser profile launch launch/patient launch/encounter", "state", iss}
	tml, _ := template.New("tml").Parse("{{.EndPoint}}?response_type=code&client_id={{.ClientID}}&redirect_uri={{.RedirectURI}}&launch={{.Launch}}&scope={{.Scope}}&state={{.State}}&aud={{.Aud}}")
	b := bytes.Buffer{}
	tml.Execute(&b, params)
	return b.String()
}

func launch(w http.ResponseWriter, r *http.Request) {
	iss := r.URL.Query()["iss"][0]
	launchID := r.URL.Query()["launch"][0]
	smartConfig, _ = getSmartConfig(iss)
	redirectURL := smartConfig.authorizationURL(iss, launchID)
	//log.Printf("redirectURL: %v", redirectURL)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func authenticate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	code := r.FormValue("code")
	client, err := smartConfig.NewSmartClient(code)
	if err != nil {
		http.Error(w, fmt.Sprintf("SmartOnFhir authentication failed with error %v", err), http.StatusBadRequest)
	}

	fmt.Fprintf(w, "SmartOnFhir authentication flow finished successfull!\nEncounter: %v", client.getEncounter())
}

func getSmartConfig(endPoint string) (*SmartConfig, error) {
	resp, err := http.Get(endPoint + "/.well-known/smart-configuration")
	if err != nil {
		return nil, fmt.Errorf("error")
	}
	config := &SmartConfig{}
	err = json.NewDecoder(resp.Body).Decode(config)
	if err != nil {
		return nil, err
	}
	config.EHREndpoint = endPoint
	return config, nil
}

func main() {
	http.HandleFunc("/launch", launch)
	http.HandleFunc("/authenticate", authenticate)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
