package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

var (
	client = createHTTPClient()
)

type Hop struct {
	Number          int
	URL             string
	StatusCode      int
	StatusCodeClass string
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

func runCmd(name string, arg ...string) {
	cmd := exec.Command(name, arg...)
	cmd.Stdout = os.Stdout
	cmd.Run()
}

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
				Number:          number,
				URL:             urlStr,
				StatusCode:      http.StatusLoopDetected,
				StatusCodeClass: getStatusCodeClass(http.StatusLoopDetected),
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
		hop.StatusCodeClass = getStatusCodeClass(resp.StatusCode)
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
				finalHop.StatusCodeClass = getStatusCodeClass(http.StatusOK)
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

func getStatusCodeClass(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	case statusCode >= 500 && statusCode < 600:
		return "5xx"
	default:
		return ""
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
	args := os.Args[1:]

	// Check if there is exactly one argument (the URL)
	if len(args) != 1 {
		fmt.Println("Usage: go-trace <URL>")
		os.Exit(1)
	}

	// Get the URL from the command-line arguments
	url := args[0]

	// Perform the trace
	redirectURL, hops, cloudflareStatus, err := followRedirects(url)
	if err != nil {
		fmt.Printf("Error tracing URL: %s\n", err)
		os.Exit(1)
	}

	// Print the trace result in tabular format
	printTraceResult(redirectURL, hops, cloudflareStatus)
}

func printTraceResult(redirectURL string, hops []Hop, cloudflareStatus bool) {
	// ANSI escape codes for bold and blue text
	boldBlue := "\033[1;34m"
	reset := "\033[0m"

	// Clear the screen
	ClearTerminal()

	fmt.Printf("%sHop%s | %sStatus%s | %sURL%s\n", boldBlue, reset, boldBlue, reset, boldBlue, reset)
	fmt.Println(strings.Repeat("-", 95))

	// Print each hop
	for _, hop := range hops {
		fmt.Fprintf(
			os.Stdout,
			"%-3d | %-6d | %s\n%s\n",
			hop.Number,
			hop.StatusCode,
			formatURL(hop.URL),
			strings.Repeat("-", 95),
		)
	}

	// Print additional information
	fmt.Fprintf(os.Stdout, "\n%sFinal URL%s:     %s\n", boldBlue, reset, formatURL(redirectURL))
	// Split the URL based on the "?" character
	parts := strings.Split(redirectURL, "?")

	if len(parts) > 1 {
		fmt.Fprintf(os.Stdout, "\n%sClean URL%s:     %s\n", boldBlue, reset, parts[0])
	}

	fmt.Println(strings.Repeat("-", 95))

}

// formatURL formats the URL for better presentation
func formatURL(url string) string {
	// Limit the width of each column
	const maxLineLength = 80

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
