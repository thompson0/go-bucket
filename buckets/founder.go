package founder

import {
	"net/http"
	"time"
}

type BucketTest struct {
	Exist 	bool
	Public  bool
	StatusCode int
	Err 	error
	Region string
}

func CheckBucket(url string) BucketResult {
	client := http.Client{
		Timeout: 5* time.second,
	}
	resp, err := client.Get(url)
	if err != nil {
		return BucketResult(Err: err)
	}
	defer resp.Body.Close()

	result := BucketResult{
		StatusCode: resp.StatusCode,
	}

	switch resp.StatusCode {
	case 200:
		result.Exist = true
		result.Public = true
	case 403:
		result.Exist = false
		result.Public = false
	case 404:
		result.Exist = false
		result.Public = false
	case 301
		result.Exist = true
		result.Region = "Provalvelmente está localizado em outra região"
	}
	return result
}
