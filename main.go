/*
    ProxyChecker - Used to get active proxies from public sources.

    Author    : Wildy Sheverando
    Email     : hai@wildyrando.com
    Git Repo  : https://github.com/wildyrando/ProxyChecker.git

    This project is licensed under the GNU Public License V3.
*/

package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Struct to store proxy IP address, port, status, and active state.
type Proxy struct {
	Ip     string
	Port   string
	Status int
	Active bool
}

// Function to check proxy status.
func checkProxy(proxy Proxy, wg *sync.WaitGroup, errChan chan error, activeFile *os.File, activeMap map[string]bool) {
	defer wg.Done()

	proxyKey := proxy.Ip + proxy.Port
	if proxy.Status == 2 && proxy.Active {
		log.Printf("[%s:%s] This proxy has already been scanned.", proxy.Ip, proxy.Port)
		return
	}

	if proxy.Status != 1 {
		log.Printf("[%s:%s] Invalid proxy.", proxy.Ip, proxy.Port)
		proxy.Status = 0 // Set status to 0 if proxy is invalid
		return
	}

	if activeMap[proxyKey] {
		log.Printf("[%s:%s] This proxy is already in the active.txt file.", proxy.Ip, proxy.Port)
		return
	}

	transport := &http.Transport{}
	proxyURL, _ := url.Parse("http://" + proxy.Ip + ":" + proxy.Port)
	transport.Proxy = http.ProxyURL(proxyURL)

	client := &http.Client{
		Transport: transport,
		Timeout:   3 * time.Second, // set timeout to 3
	}

	resp, err := client.Get("https://raw.githubusercontent.com/wildyrando/ProxyChecker/main/stt.con")
	if err != nil {
		resp, err = client.Get("https://raw.githubusercontent.com/wildyrando/ProxyChecker/main/stt.con")
	}
	if err != nil {
		log.Printf("[%s:%s] Failed connect to github.com", proxy.Ip, proxy.Port)
		proxy.Status = 0 // Set status to 0 if failed to connect to github.com
		return
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Printf("[%s:%s] Failed read response from github.com", proxy.Ip, proxy.Port)
		proxy.Status = 0 // Set status to 0 if failed to read response from github.com
		return
	}

	if strings.TrimSpace(string(body)) != "success" {
		log.Printf("[%s:%s] Response does not match expected value.", proxy.Ip, proxy.Port)
		proxy.Status = 0 // Set status to 0 if response does not match expected value
		return
	}

	log.Printf("[%s:%s] Valid proxy found.", proxy.Ip, proxy.Port)
	proxy.Active = true
	if _, err := activeFile.WriteString(proxy.Ip + ":" + proxy.Port + "\n"); err != nil {
		errChan <- fmt.Errorf("Failed write to file -> %v", err)
	}
	proxy.Status = 2 // Set status to 2 if proxy is valid and has not been scanned before
}

func main() {
	// Open active.txt file for writing and append if file already exists.
	activeFile, err := os.OpenFile("active.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("| active.txt file not found ->", err)
		return
	}
	defer activeFile.Close()

	// Create map to store active proxies.
	activeMap := make(map[string]bool)

	// Read active proxies from active.txt file and add to activeMap.
	scanner := bufio.NewScanner(activeFile)
	for scanner.Scan() {
		activeMap[scanner.Text()] = true
	}

	// Create slice to store proxy list.
	var proxyList []Proxy

	// List of URLs to retrieve proxy list from.
	urls := []string{
		"https://www.proxyscan.io/download?type=http",
		"https://www.proxyscan.io/download?type=https",
		"https://api.proxyscrape.com/?request=getproxies&proxytype=all&timeout=10000000&country=all&anonymity=all",
		"https://api.proxyscrape.com/?request=getproxies&proxytype=https&timeout=5000&country=all&ssl=all&anonymity=all",
		"https://api.proxyscrape.com/?request=getproxies&proxytype=http&timeout=5000&country=all&ssl=all&anonymity=all",
		"https://raw.githubusercontent.com/clarketm/proxy-list/master/proxy-list-raw.txt",
		"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/http.txt",
		"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/socks4.txt",
		"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/socks5.txt",
		"https://raw.githubusercontent.com/mertguvencli/http-proxy-list/main/proxy-list/data.txt",
		"https://raw.githubusercontent.com/ShiftyTR/Proxy-List/master/proxy.txt",
		"https://raw.githubusercontent.com/proxy4parsing/proxy-list/main/http.txt",
		"https://raw.githubusercontent.com/caliphdev/Proxy-List/master/http.txt",
		"https://raw.githubusercontent.com/caliphdev/Proxy-List/master/socks5.txt",
	}

	// Wait group and error channel.
	var wg sync.WaitGroup
	errChan := make(chan error)

	defer func() {
		close(errChan)
		closeActiveFileErr := activeFile.Close()
		if closeActiveFileErr != nil {
			fmt.Println("Error while closing active file:", closeActiveFileErr)
		}
	}()

	// Main looping
	for {
		// Loop to retrieve proxy list from URLs.
		for _, u := range urls {
			resp, err := http.Get(u)
			if err != nil {
				errChan <- fmt.Errorf("Failed retrieve proxy list -> %v", err)
				continue
			}
			defer resp.Body.Close()

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				errChan <- fmt.Errorf("Failed read response from %s -> %v", u, err)
				continue
			}

			// Split proxy into IP and port.
			scanner := bufio.NewScanner(strings.NewReader(string(body)))
			for scanner.Scan() {
				proxy := scanner.Text()
				if strings.Contains(proxy, "error code") {
					log.Printf("Proxy error %s", proxy)
					continue
				}

				proxySplit := strings.Split(proxy, ":")
				if len(proxySplit) < 2 {
					continue
				}

				p := Proxy{Ip: proxySplit[0], Port: proxySplit[1], Status: 1} // Set status to 1 if proxy is unscanned
				_, exist := activeMap[p.Ip+p.Port]
				if exist {
					log.Printf("[%s:%s] Proxy already scanned.", p.Ip, p.Port)
					p.Status = 2 // set the status to 2
					continue
				}

				proxyList = append(proxyList, p)
			}

			if err := scanner.Err(); err != nil {
				errChan <- fmt.Errorf("Error reading proxy list from %s -> %v", u, err)
			}
		}

		// Process proxy list concurrently.
		for i := range proxyList {
			wg.Add(1)
			go checkProxy(proxyList[i], &wg, errChan, activeFile, activeMap)
		}

		// Wait for all goroutines to finish.
		wg.Wait()

		// Clear proxy list.
		proxyList = nil

		// Write active proxies to active.txt file.
		if err := activeFile.Sync(); err != nil {
			errChan <- fmt.Errorf("Failed to sync active.txt file -> %v", err)
		}

		// Sleep for 5 minutes before retrieving proxy list again.
		time.Sleep(5 * time.Minute)
	}
}
