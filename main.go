package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/containrrr/shoutrrr"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
)

// Configuration holds environment variables
type Configuration struct {
	AccountID              string
	RuleID                 string
	CronSchedule           string
	AuthToken              string
	NotificationURL        string
	NotificationIdentifier string
	TestNotification       bool
}

// CloudflareResponse represents the response from Cloudflare API
type CloudflareResponse struct {
	Result struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		UID     string `json:"uid"`
		Include []struct {
			IP struct {
				IP string `json:"ip"`
			} `json:"ip"`
		} `json:"include"`
		Require   []interface{} `json:"require"`
		Exclude   []interface{} `json:"exclude"`
		CreatedAt string        `json:"created_at"`
		UpdatedAt string        `json:"updated_at"`
	} `json:"result"`
	Success  bool          `json:"success"`
	Errors   []interface{} `json:"errors"`
	Messages []interface{} `json:"messages"`
}

// UpdateRequest represents the update payload for Cloudflare API
type UpdateRequest struct {
	Include []struct {
		IP struct {
			IP string `json:"ip"`
		} `json:"ip"`
	} `json:"include"`
}

func loadConfig() Configuration {
	accountID := os.Getenv("ACCOUNTID")
	if accountID == "" {
		log.Fatal("ACCOUNTID environment variable is not set")
	}

	ruleID := os.Getenv("RULEID")
	if ruleID == "" {
		log.Fatal("RULEID environment variable is not set")
	}

	cronSchedule := os.Getenv("CRON")
	if cronSchedule == "" {
		log.Fatal("CRON environment variable is not set")
	}

	authToken := os.Getenv("AUTH_TOKEN")
	if authToken == "" {
		log.Fatal("AUTH_TOKEN environment variable is not set")
	}

	// Optional: Notification URL (using Shoutrrr URL format)
	notificationURL := os.Getenv("NOTIFICATION_URL")

	// Optional: Notification URL (using Shoutrrr URL format)
	notificationIdentifier := os.Getenv("NOTIFICATION_IDENTIFIER")

	// Test notification on startup (optional)
	testNotification := false
	if os.Getenv("TEST_NOTIFICATION") == "true" {
		testNotification = true
	}

	return Configuration{
		AccountID:              accountID,
		RuleID:                 ruleID,
		CronSchedule:           cronSchedule,
		AuthToken:              authToken,
		NotificationURL:        notificationURL,
		NotificationIdentifier: notificationIdentifier,
		TestNotification:       testNotification,
	}
}

func getCurrentIP() (string, error) {
	// List of IP service providers to try in order
	ipProviders := []struct {
		URL      string
		JsonPath string // Empty for plain text response
	}{
		{"https://api.ipify.org?format=json", "ip"},
		{"https://api.my-ip.io/ip.json", "ip"},
		{"https://ifconfig.me/all.json", "ip_addr"},
		{"https://ipinfo.io/json", "ip"},
		{"https://api.myip.com", "ip"},
		{"https://ifconfig.co/json", "ip"},
		{"https://ip.seeip.org/jsonip", "ip"},
		{"https://icanhazip.com", ""},    // Plain text
		{"https://ifconfig.me", ""},      // Plain text
		{"https://ipecho.net/plain", ""}, // Plain text
	}

	var lastError error
	client := &http.Client{
		Timeout: 5 * time.Second, // Set timeout to avoid hanging
	}

	for _, provider := range ipProviders {
		log.Printf("Trying to get IP from: %s", provider.URL)

		resp, err := client.Get(provider.URL)
		if err != nil {
			log.Printf("Failed to get IP from %s: %v", provider.URL, err)
			lastError = err
			continue
		}

		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				log.Printf("Failed to close response body from %s: %v", provider.URL, err)
			}
		}(resp.Body)

		// Check if we got a successful response
		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			log.Printf("Failed to get IP from %s: Status %d, Body: %s", provider.URL, resp.StatusCode, string(bodyBytes))
			lastError = fmt.Errorf("HTTP error: %d", resp.StatusCode)
			continue
		}

		// Handle JSON response
		if provider.JsonPath != "" {
			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				log.Printf("Failed to decode JSON from %s: %v", provider.URL, err)
				lastError = err
				continue
			}

			// Extract IP from the specified JSON path
			if ipValue, ok := result[provider.JsonPath]; ok {
				if ipStr, ok := ipValue.(string); ok && ipStr != "" {
					log.Printf("Successfully obtained IP from %s", provider.URL)
					return ipStr, nil
				}
			}

			lastError = fmt.Errorf("could not find IP in JSON response from %s", provider.URL)
			continue
		} else {
			// Handle plain text response
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("Failed to read response from %s: %v", provider.URL, err)
				lastError = err
				continue
			}

			ip := strings.TrimSpace(string(bodyBytes))
			// Basic validation: check that we have something that looks like an IP
			if ip != "" && strings.Contains(ip, ".") {
				log.Printf("Successfully obtained IP from %s", provider.URL)
				return ip, nil
			}

			lastError = fmt.Errorf("received invalid IP from %s: %s", provider.URL, ip)
		}
	}

	return "", fmt.Errorf("all IP providers failed, last error: %v", lastError)
}

func getCloudflareGroup(config Configuration) (*CloudflareResponse, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/access/groups/%s", config.AccountID, config.RuleID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+config.AuthToken)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get Cloudflare group: %s, status: %d", string(bodyBytes), resp.StatusCode)
	}

	var cfResponse CloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResponse); err != nil {
		return nil, err
	}

	return &cfResponse, nil
}

func updateCloudflareGroup(config Configuration, newIP string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/access/groups/%s", config.AccountID, config.RuleID)

	updateReq := UpdateRequest{
		Include: []struct {
			IP struct {
				IP string `json:"ip"`
			} `json:"ip"`
		}{
			{
				IP: struct {
					IP string `json:"ip"`
				}{
					IP: newIP + "/32",
				},
			},
		},
	}

	jsonData, err := json.Marshal(updateReq)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", "Bearer "+config.AuthToken)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update Cloudflare group: %s, status: %d", string(bodyBytes), resp.StatusCode)
	}

	return nil
}

// sendNotification sends a notification using Shoutrrr if configured
func sendNotification(config Configuration, message string) error {
	if config.NotificationURL == "" {
		log.Println("Notification URL not configured, skipping notification")
		return nil
	}

	log.Printf("Sending notification: %s", message)

	// Adding Identifier to the message
	msg := fmt.Sprintf("%s: %s", config.NotificationIdentifier, message)

	err := shoutrrr.Send(config.NotificationURL, msg)
	if err != nil {
		return fmt.Errorf("failed to send notification: %v", err)
	}

	log.Println("Notification sent successfully")
	return nil
}

// startHealthCheckServer starts a simple HTTP server for container health checks
func startHealthCheckServer(port string) {
	// Check if the port is empty
	if port == "" {
		port = "8080"
	}

	// Define a simple handler for health checks
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		if err != nil {
			return
		}
	})

	// Define a handler for readiness checks that provides more details
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		info := map[string]interface{}{
			"status":    "OK",
			"timestamp": time.Now().Format(time.RFC3339),
			"uptime":    time.Since(startTime).String(),
		}

		jsonData, err := json.Marshal(info)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(jsonData)
		if err != nil {
			return
		}
	})

	// Start the HTTP server in a goroutine
	go func() {
		addr := fmt.Sprintf(":%s", port)
		log.Printf("Starting health check server on %s", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("Health check server error: %v", err)
		}
	}()
}

// Global variable to track application start time
var startTime time.Time

func checkAndUpdateIP(config Configuration) {
	log.Println("Checking if IP update is needed...")

	// Get current public IP
	currentIP, err := getCurrentIP()
	if err != nil {
		log.Printf("Error getting current IP: %v", err)
		// Notify about error
		if config.NotificationURL != "" {
			err := sendNotification(config, fmt.Sprintf("❌ Error getting current IP: %v", err))
			if err != nil {
				return
			}
		}
		return
	}
	currentIP = strings.TrimSpace(currentIP)
	log.Printf("Current public IP: %s", currentIP)

	// Get Cloudflare Access Group
	cfGroup, err := getCloudflareGroup(config)
	if err != nil {
		log.Printf("Error getting Cloudflare Access Group: %v", err)
		// Notify about error
		if config.NotificationURL != "" {
			err := sendNotification(config, fmt.Sprintf("❌ Error getting Cloudflare Access Group: %v", err))
			if err != nil {
				return
			}
		}
		return
	}

	// Check if there's at least one IP in the include list
	if len(cfGroup.Result.Include) == 0 || cfGroup.Result.Include[0].IP.IP == "" {
		log.Println("No IP found in Cloudflare Access Group, updating...")
		err = updateCloudflareGroup(config, currentIP)
		if err != nil {
			log.Printf("Error updating Cloudflare Access Group: %v", err)
			// Notify about error
			if config.NotificationURL != "" {
				err := sendNotification(config, fmt.Sprintf("❌ Error updating Cloudflare Access Group: %v", err))
				if err != nil {
					return
				}
			}
		} else {
			log.Printf("Successfully updated Cloudflare Access Group with IP: %s", currentIP)
			// Notify about successful update
			if config.NotificationURL != "" {
				err := sendNotification(config, fmt.Sprintf("✅ Initial IP set in Cloudflare Access Group: %s", currentIP))
				if err != nil {
					return
				}
			}
		}
		return
	}

	// Get the IP from Cloudflare (remove /32 suffix if present)
	cfIP := cfGroup.Result.Include[0].IP.IP
	cfIP = strings.TrimSuffix(cfIP, "/32")
	log.Printf("Cloudflare Access Group IP: %s", cfIP)

	// Compare IPs
	if currentIP != cfIP {
		log.Printf("IP mismatch detected. Updating Cloudflare Access Group from %s to %s", cfIP, currentIP)
		err = updateCloudflareGroup(config, currentIP)
		if err != nil {
			log.Printf("Error updating Cloudflare Access Group: %v", err)
			// Notify about error
			if config.NotificationURL != "" {
				err := sendNotification(config, fmt.Sprintf("❌ Failed to update IP from %s to %s: %v", cfIP, currentIP, err))
				if err != nil {
					return
				}
			}
		} else {
			log.Printf("Successfully updated Cloudflare Access Group with IP: %s", currentIP)
			// Notify about successful update
			if config.NotificationURL != "" {
				err := sendNotification(config, fmt.Sprintf("🔄 IP Address Updated: %s ➡️ %s", cfIP, currentIP))
				if err != nil {
					return
				}
			}
		}
	} else {
		log.Println("IP is already up to date, no action needed")
	}
}

func main() {
	// Initialize the start time for uptime tracking
	startTime = time.Now()

	log.Println("Cloudflare Access Group IP Updater")

	// Load the.env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading it. Using environment variables directly.")
	} else {
		log.Println("Successfully loaded .env file")
	}

	// Load configuration
	config := loadConfig()

	// Start the health check server
	startHealthCheckServer("8080")

	// Send test notification if requested
	if config.TestNotification && config.NotificationURL != "" {
		log.Println("Sending test notification...")
		err := sendNotification(config, "🚀 Cloudflare IP Updater started - Test notification")
		if err != nil {
			log.Printf("Test notification failed: %v", err)
		} else {
			log.Println("Test notification sent successfully")
		}
	}

	// Run once immediately
	checkAndUpdateIP(config)

	// Setup cron scheduler
	c := cron.New()
	_, err := c.AddFunc(config.CronSchedule, func() {
		checkAndUpdateIP(config)
	})

	if err != nil {
		log.Fatalf("Error setting up cron job: %v", err)
	}

	c.Start()

	log.Printf("Cloudflare IP Updater running on schedule: %s", config.CronSchedule)

	// Wait for the termination signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig

	c.Stop()

	// Send notification on shutdown if configured
	if config.NotificationURL != "" {
		err := sendNotification(config, "⏹️ Cloudflare IP Updater stopped")
		if err != nil {
			return
		}
	}

	log.Println("Cloudflare IP Updater stopped")
}
