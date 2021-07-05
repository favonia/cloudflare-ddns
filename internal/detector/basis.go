package detector

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"time"
)

func getIPFromCloudflare(url string, timeout time.Duration) (net.IP, error) {
	client := http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("😩 Could not connect to the CloudFlare server: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`(?m:^ip=(.*)$)`)
	ms := re.FindSubmatch(body)
	if ms == nil {
		return nil, fmt.Errorf(`😩 Could not find "ip=..." in the response: %q.`, string(body))
	}

	return net.ParseIP(string(ms[1])), nil
}
