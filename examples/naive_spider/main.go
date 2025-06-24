package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"sync"
	"syscall"
	"time"

	"github.com/626D6172/flow"
)

var seen = map[string]struct{}{} // To keep track of visited URLs
var seenLock sync.Mutex

func markSeen(url string) {
	seenLock.Lock()
	defer seenLock.Unlock()
	seen[url] = struct{}{}
}

func isSeen(url string) bool {
	seenLock.Lock()
	defer seenLock.Unlock()
	_, exists := seen[url]
	return exists
}

func downloader(ctx context.Context, inters ...interface{}) ([]interface{}, error) {
	out := make([]interface{}, 0, len(inters))
	for _, inter := range inters {
		url, ok := inter.(string)
		if !ok {
			return nil, errors.New("expected string input representing a URL")
		}

		log.Default().Printf("Downloading URL: %s\n", url)

		// Download the contents of the URL
		client := &http.Client{
			Timeout: 2 * time.Second,
		}
		resp, err := client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to download URL %s: %w", url, err)
		}
		defer resp.Body.Close()

		// Check if Content-Type starts with "text/html"
		if contentType := resp.Header.Get("Content-Type"); !regexp.MustCompile(`^text/html`).MatchString(contentType) {
			log.Default().Printf("Skipping URL %s: Content-Type %s is not HTML\n", url, contentType)
			continue // Skip non-HTML content
		}

		// Read the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body for URL %s: %w", url, err)
		}

		log.Default().Printf("Downloaded %d bytes from URL: %s\n", len(body), url)
		markSeen(url)
		out = append(out, string(body)) // Append the contents as a string
	}

	return out, nil
}

func scanner(ctx context.Context, inters ...interface{}) ([]interface{}, error) {
	out := make([]interface{}, 0, len(inters))
	re := `https?://[a-zA-Z0-9\-._~:/?#\[\]@!$&'()*+,;=%]+`
	urlRegex := regexp.MustCompile(re)
	for _, inter := range inters {
		content, ok := inter.(string)
		if !ok {
			return nil, errors.New("expected string input representing downloaded content")
		}

		// Use a regular expression to find all valid URLs in the content
		// Use a stricter regular expression to find URLs within HTML strings
		matches := urlRegex.FindAllString(content, -1)

		log.Default().Printf("Found %d URLs in content\n", len(matches))
		// Append the matches to the output
		for _, match := range matches {
			out = append(out, match)
		}
	}

	return out, nil
}

func crawler(ctx context.Context, inters ...interface{}) ([]interface{}, error) {
	for _, inter := range inters {
		if url, ok := inter.(string); ok {
			log.Default().Printf("Discovered URL: %s\n", url)
		}
	}
	return inters, nil
}

func printer(ctx context.Context, inters ...interface{}) ([]interface{}, error) {
	for _, inter := range inters {
		log.Default().Printf("Found URL: %s\n", inter)
	}
	return inters, nil
}

func errorMethod(ctx context.Context, err error, inters ...interface{}) {
	// Handle error, e.g., log it or send it to a monitoring system.
	log.Default().Printf("Error in adder node: %v with inputs: %v\n", err, inters)
}

func main() {
	downloader := &flow.Node{
		Method:       downloader,
		WorkersCount: 1,
		ErrorMethod:  errorMethod,
	}

	scanner := &flow.Node{
		Method:       scanner,
		WorkersCount: 2,
		ErrorMethod:  errorMethod,
	}

	crawler := &flow.Node{
		Method:              crawler,
		WorkersCount:        1,
		ErrorMethod:         errorMethod,
		EnableGoRoutineSend: true,
		ValidFunc: func(ctx context.Context, inters ...interface{}) bool {
			// Validate that all inputs are strings representing URLs before passing to the downloader node
			for _, inter := range inters {
				url, ok := inter.(string)
				if !ok {
					return false
				}

				if isSeen(url) {
					log.Default().Printf("Skipping already seen URL: %s\n", url)
					return false
				}
			}
			return true
		},
	}

	printer := &flow.Node{
		Method:       printer,
		WorkersCount: 1,
	}

	// Downloader will download the content from the URLs and send it to the pool node.
	downloader.Link(scanner)

	// scanner will push all URLs to the crawler and printer node.
	// The crawler node will send the URLs to the downloader node for processing.
	scanner.Link(crawler)
	crawler.Link(downloader, printer)

	// Create a context that is bound by system halt (e.g., interrupt signal)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Listen for system interrupt signals to gracefully shut down
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
		log.Default().Printf("Received interrupt signal, shutting down...\n")
		cancel()
	}()
	downloader.Start(ctx)

	go func(ctx context.Context) {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				log.Default().Printf("Downloader Queue length: %d\n", len(downloader.Queue))
				log.Default().Printf("Scanner Queue length: %d\n", len(scanner.Queue))
				log.Default().Printf("Crawler Queue length: %d\n", len(crawler.Queue))
				log.Default().Printf("Printer Queue length: %d\n", len(printer.Queue))
			}
		}

	}(ctx)

	// Send the seed url
	downloader.Send("https://example.com")
	downloader.Wait()
}
