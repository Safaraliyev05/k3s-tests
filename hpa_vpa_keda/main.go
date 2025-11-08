package main

import (
	"net/http"
	"io"
	"log"
	"fmt"
	"sync"
	"time"
)

func fetchData(url string) {
	response, err := http.Get(url)
	if err != nil {
		log.Printf("Error fetching %s: %v", url, err)
		return
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("Error reading response from %s: %v", url, err)
		return
	}
	fmt.Printf("Response from %s (length: %d bytes)\n", url, len(body))
}

func main() {
	wg := sync.WaitGroup{}

	// Student service IP addresses for autoscaling testing
	studentIPs := []string{
		"http://10.42.0.33:8080/",
	}

	startTime := time.Now()
	fmt.Printf("Starting autoscaling test at %v\n", startTime)

	// Phase 1: High load for first 10 seconds to trigger scale-up
	fmt.Println("Phase 1: High load (10 seconds) - triggering scale-up...")
	go func() {
		for time.Since(startTime) < 10*time.Minute {
			for _, ip := range studentIPs {
				wg.Add(1)
				go func(url string) {
					defer wg.Done()
					fetchData(url)
				}(ip)
			}
			time.Sleep(50 * time.Millisecond) // Very frequent requests
		}
	}()

	// Wait for the high load phase to complete
	time.Sleep(10 * time.Minute)

	fmt.Println("Phase 2: Reduced load - allowing scale-down...")
	// Phase 2: Reduced load to trigger scale-down
	for i := 0; i < 10; i++ {
		for _, ip := range studentIPs {
			wg.Add(1)
			go func(url string) {
				defer wg.Done()
				fetchData(url)
			}(ip)
		}
		time.Sleep(5 * time.Second) // Much slower requests
	}

	fmt.Println("Waiting for all requests to complete...")
	wg.Wait()
	fmt.Printf("Autoscaling test completed. Total duration: %v\n", time.Since(startTime))
}