package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"
)

var localWebPort string
var onewireDeviceID string
var cloudflareEmail string
var cloudflareAccountID string
var cloudflareNamespaceID string
var cloudflareAuthKey string
var cloudflareKVExpiration string

func init() {
	flag.StringVar(&localWebPort, "port", ":8080", "Port for Raspberry Pi to serve temperature on local network (default ':8080')")
	flag.StringVar(&cloudflareEmail, "email", "", "Email used for Cloudflare account")
	flag.StringVar(&cloudflareAccountID, "account", "", "Cloudflare account id to send temperatures to Cloudflare KV")
	flag.StringVar(&cloudflareNamespaceID, "namespace", "PiTemperature", "Sets the Cloudflare namespace for the KV")
	flag.StringVar(&cloudflareAuthKey, "authkey", "", "The auth key for the namespace")
	flag.StringVar(&cloudflareKVExpiration, "expiration", "3600", "Seconds until the key/value pair expires (default: 3600)")
	flag.StringVar(&onewireDeviceID, "device", "", "Onewire device id")
}

func main() {
	flag.Parse()
	fmt.Println(localWebPort, onewireDeviceID, cloudflareEmail, cloudflareAccountID, cloudflareNamespaceID, cloudflareAuthKey, cloudflareKVExpiration)

	http.HandleFunc("/", tempToWeb)
	go pollTemp()
	log.Fatal(http.ListenAndServe(localWebPort, nil))
}

func pollTemp() {
	for range time.Tick(30 * time.Second) {
		c, _, err := getTemp()
		if err != nil {
			break
		}
		writeToKV(c)
	}
}

func getTemp() (c float64, f float64, err error) {
	tempFile, err := ioutil.ReadFile(fmt.Sprintf("/sys/bus/w1/devices/%s/hwmon/hwmon1/temp1_input", onewireDeviceID))
	fmt.Println(tempFile)
	fmt.Println([]byte("\n"))
	if err != nil {
		fmt.Println("couldn't read tmpfile")
		return 0, 0, err
	}

	raw, err := strconv.ParseInt(string(tempFile[:len(tempFile)-1]), 10, 0)
	if err != nil {
		return 0, 0, err
	}
	c = float64(raw) * 0.001
	fmt.Printf("C: %v\ntempFile: %v\nraw: %v\n", c, string(tempFile), raw)
	f = c*1.8 + 32.0
	return c, f, nil
}

func writeToKV(c float64) {
	hc := http.Client{}
	timestamp := time.Now()
	buf := []byte(fmt.Sprintf("%v", c))
	req, err := http.NewRequest("PUT", fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/storage/kv/namespaces/%s/values/%s?expiration_ttl=%s", cloudflareAccountID, cloudflareNamespaceID, timestamp, cloudflareKVExpiration), bytes.NewBuffer(buf))
	req.Header.Add("X-Auth-Email", cloudflareEmail)
	req.Header.Add("X-Auth-Key", cloudflareAuthKey)
	resp, err := hc.Do(req)
	if err != nil {
		log.Fatal("OOoops")
	}
	defer resp.Body.Close()
	fmt.Println("Response status: ", resp.Status)
	fmt.Println("Response Headers: ", resp.Header)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error Reading body. ", err)
	}
	fmt.Printf("%s\n", body)
}

func tempToWeb(w http.ResponseWriter, r *http.Request) {
	c, f, err := getTemp()
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		w.Write([]byte("WaaWaaWaa"))
	}
	w.Write([]byte(fmt.Sprintf("%v Degrees F | %v Degrees C", f, c)))
}
