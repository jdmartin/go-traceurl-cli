package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pelletier/go-toml"
)

var (
	client             = createHTTPClient()
	outputWidth        = 120
	outputDividerWidth = 135
	green              = "\033[32m"
	boldBlue           = "\033[1;34m"
	underline          = "\033[4m"
	reset              = "\033[0m"
)

// Config struct to hold configuration values
type Config struct {
	UseJSON       bool `toml:"use_json"`
	AlwaysTerse   bool `toml:"always_terse"`
	AlwaysVerbose bool `toml:"always_verbose"`
	Width         int  `toml:"width"`
}

type Hop struct {
	Number     int
	URL        string
	StatusCode int
}

type OutputPath struct {
	Path string
}

type TraceResult struct {
	Hops     []Hop  `json:"hops"`
	FinalURL string `json:"finalURL"`
	CleanURL string `json:"cleanURL"`
}

// Utility Functions
func ClearTerminal() {
	switch runtime.GOOS {
	case "darwin":
		runCmd("clear")
	case "linux":
		runCmd("clear")
	case "windows":
		runCmd("cmd", "/c", "cls")
	default:
		runCmd("clear")
	}
}

func createHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			ResponseHeaderTimeout: 5 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Stop following redirects after the first hop
			if len(via) >= 1 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

// formatURL formats the URL for better presentation
func formatURL(url string) string {
	// Limit the width of each column
	var maxLineLength = outputWidth

	if len(url) <= maxLineLength {
		return url
	}

	var formattedURL strings.Builder

	lineStart := 0
	for i := 0; i < len(url); i += maxLineLength {
		end := i + maxLineLength
		if end > len(url) {
			end = len(url)
		}

		if i > 0 {
			// Insert additional indentation for the URL continuation
			formattedURL.WriteString("\n" + strings.Repeat(" ", 15))
		}

		formattedURL.WriteString(url[lineStart:end])
		lineStart = end
	}

	return formattedURL.String()
}

// loadConfig reads the configuration file and returns a Config struct
// If the file doesn't exist or some values are missing, default values are used
func loadConfig() (*Config, error) {
	// Check if XDG_CONFIG_HOME is set
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	var configDir string

	if xdgConfigHome != "" {
		configDir = xdgConfigHome
	} else {
		// Get user's home directory
		usr, err := user.Current()
		if err != nil {
			return nil, err
		}
		configDir = filepath.Join(usr.HomeDir, ".config")
	}

	// Define config file path in the selected directory
	configFilePath := filepath.Join(configDir, "go-trace.toml")

	// Read the config file
	file, err := os.ReadFile(configFilePath)
	if os.IsNotExist(err) {
		// go-trace.toml does not exist, return nil config (no error)
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	// Unmarshal TOML content into Config struct
	var config Config
	err = toml.Unmarshal(file, &config)
	if err != nil {
		return nil, err
	}

	// Set default values if not present
	if config.UseJSON == false {
		config.UseJSON = false // Set the default value
	}
	if config.AlwaysTerse == false {
		config.AlwaysTerse = false // Set the default value
	}
	if config.AlwaysVerbose == false {
		config.AlwaysVerbose = false // Set the default value
	}
	if config.Width == 0 {
		config.Width = 120 // Set the default value
	}

	return &config, nil
}

// Try to make a clean URL
func makeCleanURL(url string) string {
	// Split the URL based on the "?" character
	parts := strings.Split(url, "?")

	if len(parts) > 1 {
		return parts[0]
	} else {
		return url
	}
}

// Output as JSON
func outputAsJSON(traceResult TraceResult) error {
	// Marshal the TraceResult struct into a formatted JSON string
	jsonString, err := json.MarshalIndent(traceResult, "", "  ")
	if err != nil {
		return err
	}

	// Print the JSON string
	fmt.Println(string(jsonString))

	return nil
}

func printUsageMessage() {
	fmt.Printf("\n%sUsage%s: go-trace [options] <URL>\n\n\t%sOptions%s:\n\t-h: prints this help message\n\t-j: outputs as JSON\n\t-s: prints only the final/clean URL\n\t-v: shows all hops\n\t-w: sets the width of the URL tab (line wraps here)\n\n\t%sDefaults%s:\n\t-j: Off\n\t-v: Off (Final/Clean URL only)\n\t-w: 120\n\n", underline, reset, underline, reset, underline, reset)
}

func printShortTraceResult(redirectURL string) {
	// Print additional information
	fmt.Fprintf(os.Stdout, "\n%sFinal URL%s:     %s\n", boldBlue, reset, formatURL(redirectURL))

	cleanedURL := makeCleanURL(redirectURL)

	if cleanedURL != redirectURL {
		fmt.Fprintf(os.Stdout, "\n%sClean URL%s:     %s\n", green, reset, cleanedURL)
	}

	fmt.Printf("\n")
}

func printTerseTraceResult(redirectURL string) {
	cleanedURL := makeCleanURL(redirectURL)

	if cleanedURL != redirectURL {
		fmt.Println(cleanedURL)
	} else {
		fmt.Println(redirectURL)
	}
}

func printVerboseTraceResult(redirectURL string, hops []Hop, cloudflareStatus bool) {
	fmt.Printf("%sHop%s | %sStatus%s | %sURL%s\n", boldBlue, reset, boldBlue, reset, boldBlue, reset)
	fmt.Println(strings.Repeat("-", outputDividerWidth))

	// Print each hop
	for _, hop := range hops {
		fmt.Fprintf(
			os.Stdout,
			"%-3d | %-6d | %s\n%s\n",
			hop.Number,
			hop.StatusCode,
			formatURL(hop.URL),
			strings.Repeat("-", outputDividerWidth),
		)
	}

	// Print additional information
	fmt.Fprintf(os.Stdout, "\n%sFinal URL%s:     %s\n", boldBlue, reset, formatURL(redirectURL))

	cleanedURL := makeCleanURL(redirectURL)

	if cleanedURL != redirectURL {
		fmt.Fprintf(os.Stdout, "\n%sClean URL%s:     %s\n", green, reset, cleanedURL)
	}

	fmt.Println(strings.Repeat("-", outputDividerWidth))
}

func runCmd(name string, arg ...string) {
	cmd := exec.Command(name, arg...)
	cmd.Stdout = os.Stdout
	cmd.Run()
}

// Tracer Functions

func doTimeout() {
	fmt.Println("Timeout error")
	os.Exit(1)
}

func doValidationError() {
	fmt.Println("Validation error")
	os.Exit(1)
}

func followRedirects(urlStr string) (string, []Hop, bool, error) {
	// CF didn't break anything yet.
	var cloudflareStatus bool // Defaults to false

	hops := []Hop{}
	number := 1

	var previousURL *url.URL

	// Use a set to keep track of visited URLs
	visitedURLs := make(map[string]int)

	// Ensure the initial URL is marked as visited
	visitedURLs[urlStr] = 1

	for {
		// Check if the URL has been visited before
		if visitedURLs[urlStr] > 1 {
			// Redirect loop detected
			hops = append(hops, Hop{
				Number:     number,
				URL:        urlStr,
				StatusCode: http.StatusLoopDetected,
			})
			return urlStr, hops, cloudflareStatus, nil
		} else {
			visitedURLs[urlStr]++
		}

		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			return "", nil, cloudflareStatus, fmt.Errorf("error creating request: %s", err)
		}

		// Set the user agent header
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

		resp, err := client.Do(req)
		if err != nil {
			if err, ok := err.(*url.Error); ok && err.Timeout() {
				doTimeout()
				return "", nil, cloudflareStatus, nil
			}

			if strings.Contains(err.Error(), "x509: certificate signed by unknown authority") {
				// Handle certificate verification error
				doValidationError()
				return "", nil, cloudflareStatus, nil
			}

			// Close response body in case of error
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}

			return "", nil, cloudflareStatus, fmt.Errorf("error accessing URL: %s", err)
		}

		if resp != nil && resp.Body != nil {
			defer resp.Body.Close()
		}

		hop := Hop{
			Number:     number,
			URL:        urlStr,
			StatusCode: resp.StatusCode,
		}
		hops = append(hops, hop)

		if resp.StatusCode >= 300 && resp.StatusCode <= 399 {
			location := resp.Header.Get("Location")
			if location == "" {
				if strings.Contains(resp.Header.Get("Server"), "cloudflare") {
					cloudflareStatus = true
				}
				return "", []Hop{}, cloudflareStatus, nil // Return empty slice of Hop when redirect location is not found
			}
			if strings.HasPrefix(location, "https://outlook.office365.com") {
				// Only include the final request as the last hop
				finalHop := Hop{
					Number:     number + 2, // Increment the hop number for the final request
					URL:        location,
					StatusCode: http.StatusOK, // Set the status code to 200 for the final request
				}
				hops = append(hops, finalHop)

				return location, hops, cloudflareStatus, nil
			}

			redirectURL, err := handleRelativeRedirect(previousURL, location, req.URL)
			if err != nil {
				return "", nil, cloudflareStatus, fmt.Errorf("error handling relative redirect: %s", err)
			}

			// Convert redirectURL to a string
			redirectURLString := redirectURL.String()

			// Check if the "returnUri" query parameter is present
			u, err := url.Parse(redirectURLString)
			if err != nil {
				return "", nil, cloudflareStatus, fmt.Errorf("error parsing URL: %s", err)
			}
			queryParams := u.Query()
			if returnURI := queryParams.Get("returnUri"); returnURI != "" {
				decodedReturnURI, err := url.PathUnescape(returnURI)
				if err != nil {
					return "", nil, cloudflareStatus, fmt.Errorf("error decoding returnUri: %s", err)
				}
				decodedReturnURI = strings.ReplaceAll(decodedReturnURI, "%3A", ":")
				decodedReturnURI = strings.ReplaceAll(decodedReturnURI, "%2F", "/")

				redirectURLString = u.Scheme + "://" + u.Host + u.Path + "?returnUri=" + decodedReturnURI
			}

			if redirURI := queryParams.Get("redir"); redirURI != "" {
				decodedRedirURI, err := url.PathUnescape(redirURI)
				if err != nil {
					return "", nil, cloudflareStatus, fmt.Errorf("error decoding redir param: %s", err)
				}
				decodedRedirURI = strings.ReplaceAll(decodedRedirURI, "%3A", ":")
				decodedRedirURI = strings.ReplaceAll(decodedRedirURI, "%2F", "/")

				redirectURLString = u.Scheme + "://" + u.Host + u.Path + "?redir=" + decodedRedirURI
			}

			urlStr = redirectURLString
			number++

			previousURL, err = url.Parse(urlStr)
			if err != nil {
				return "", nil, cloudflareStatus, fmt.Errorf("error parsing URL: %s", err)
			}
			continue
		}

		return urlStr, hops, cloudflareStatus, nil
	}
}

func handleRelativeRedirect(previousURL *url.URL, location string, requestURL *url.URL) (*url.URL, error) {
	redirectURL, err := url.Parse(location)
	if err != nil {
		log.Printf("Error parsing redirect URL: %v", err)
		return nil, err
	}

	if redirectURL.Scheme == "" {
		// If the scheme is missing, set it to the scheme of the previous URL or the request URL
		if previousURL != nil {
			redirectURL.Scheme = previousURL.Scheme
		} else if requestURL != nil {
			redirectURL.Scheme = requestURL.Scheme
		} else {
			return nil, errors.New("missing scheme for relative redirect")
		}
	}

	if redirectURL.Host == "" {
		// If the host is missing, set it to the host of the previous URL or the request URL
		if previousURL != nil {
			redirectURL.Host = previousURL.Host
		} else if requestURL != nil {
			redirectURL.Host = requestURL.Host
		} else {
			return nil, errors.New("missing host for relative redirect")
		}
	}

	return redirectURL, nil
}

func main() {
	// Parse command-line arguments
	var (
		flagHelp       bool
		flagOutputJSON bool
		flagTerse      bool
		flagVerbose    bool
		flagWidth      int
	)

	flag.BoolVar(&flagHelp, "h", false, "Show help message")
	flag.BoolVar(&flagHelp, "help", false, "Show help message")
	flag.BoolVar(&flagOutputJSON, "j", false, "Output results as JSON")
	flag.BoolVar(&flagTerse, "s", false, "Output only the final/clean url")
	flag.BoolVar(&flagVerbose, "v", false, "Show verbose trace results")
	flag.IntVar(&flagWidth, "w", 120, "Width of the URL tab")

	// Load configuration from file, if exists
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading configuration: %s\n", err)
		os.Exit(1)
	}

	// Set flag values based on config, or use default values if config is nil
	if config != nil {
		flagOutputJSON = config.UseJSON
		flagTerse = config.AlwaysTerse
		flagVerbose = config.AlwaysVerbose
		flagWidth = config.Width
	}

	flag.Parse()
	args := flag.Args()

	// Check if there are additional arguments after the URL
	if len(args) < 1 {
		printUsageMessage()
		os.Exit(1)
	}

	// Get the URL from the command-line arguments
	url := args[0]

	// Check if there are flags after the URL
	if len(args) > 1 {
		printUsageMessage()
		os.Exit(1)
	}

	// If help requested, print message and exit
	if flagHelp {
		printUsageMessage()
		os.Exit(0)
	}

	// Perform the trace
	redirectURL, hops, cloudflareStatus, err := followRedirects(url)
	if err != nil {
		fmt.Printf("Error tracing URL: %s\n", err)
		os.Exit(1)
	}

	traceResult := TraceResult{
		Hops:     hops,
		FinalURL: redirectURL,
		CleanURL: makeCleanURL(redirectURL),
	}

	// Change URL tab width, if required.
	if flagWidth != 120 {
		outputWidth = flagWidth
		outputDividerWidth = flagWidth + 15
	}

	// Save to JSON if requested
	if flagOutputJSON {
		outputAsJSON(traceResult)
		os.Exit(0)
	}

	// Clear the screen before printing results
	ClearTerminal()

	// Print the trace result in terse or tabular format
	if flagTerse {
		printTerseTraceResult(redirectURL)
		os.Exit(0)
	} else if flagVerbose {
		printVerboseTraceResult(redirectURL, hops, cloudflareStatus)
	} else {
		printShortTraceResult(redirectURL)
	}
}
