package buckets

import (
	"fmt"
	"net/http"
	"time"
)

type BucketTest struct {
	Exist      bool
	Public     bool
	StatusCode int
	Err        error
	Region     string
	Response   string 
}

func CheckBucket(url string, debug bool) BucketTest {
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		if debug {
			fmt.Printf("[DEBUG] %s | Error: %v\n", url, err)
		}
		return BucketTest{Err: err}
	}

	resp, err := client.Do(req)
	if err != nil {
		if debug {
			fmt.Printf("[DEBUG] %s | Error: %v\n", url, err)
		}
		return BucketTest{Err: err}
	}
	defer resp.Body.Close()

	result := BucketTest{
		StatusCode: resp.StatusCode,
	}

	region := resp.Header.Get("x-amz-bucket-region")
	if region != "" {
		result.Region = region
	}

	switch resp.StatusCode {
	case 200:
		result.Exist = true
		result.Public = true
	case 403:
		result.Exist = true 
		result.Public = false
	case 404:
		result.Exist = false
	case 301:
		result.Exist = true
	default:
		
	}

	if debug == true {
		fmt.Printf("[DEBUG] %s | Status: %d | Region: %s | Exist: %v | Public: %v\n", 
			url, resp.StatusCode, region, result.Exist, result.Public)
	}

	return result
}