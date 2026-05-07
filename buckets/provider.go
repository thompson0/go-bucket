package buckets

import (
	"fmt"
	"net/url"
	"strings"
)

type Provider string

const (
	ProviderAWS   Provider = "aws"
	ProviderAzure Provider = "azure"
)

func ParseProvider(input string) (Provider, error) {
	switch Provider(strings.ToLower(strings.TrimSpace(input))) {
	case ProviderAWS:
		return ProviderAWS, nil
	case ProviderAzure:
		return ProviderAzure, nil
	default:
		return "", fmt.Errorf("provedor inválido: %s", input)
	}
}

func (p Provider) hostSuffix() string {
	switch p {
	case ProviderAzure:
		return ".blob.core.windows.net"
	default:
		return ".s3.amazonaws.com"
	}
}

func (p Provider) ResourceURL(name string) string {
	return fmt.Sprintf("https://%s%s/", name, p.hostSuffix())
}

func FormatBucketURL(input string, provider Provider) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return input
	}

	if strings.Contains(strings.ToLower(input), provider.hostSuffix()) {
		return input
	}

	if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") {
		input = "https://" + input
	}

	u, err := url.Parse(input)
	if err != nil {
		return provider.ResourceURL(strings.ToLower(strings.TrimSpace(input)))
	}

	if u.Host == "" {
		return provider.ResourceURL(strings.ToLower(strings.TrimSpace(input)))
	}

	return provider.ResourceURL(strings.ToLower(strings.TrimSpace(u.Host)))
}

func NormalizeBucketURL(input string, provider Provider) string {
	formatted := FormatBucketURL(input, provider)
	formatted = strings.TrimSpace(formatted)

	u, err := url.Parse(formatted)
	if err != nil || u.Host == "" {
		return formatted
	}

	host := strings.ToLower(strings.TrimSpace(u.Host))
	host = strings.TrimSuffix(host, "/")

	return fmt.Sprintf("https://%s/", host)
}

func ResourceName(input string, provider Provider) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return input
	}

	if u, err := url.Parse(input); err == nil && u.Host != "" {
		input = u.Host
	}

	input = strings.TrimPrefix(input, "http://")
	input = strings.TrimPrefix(input, "https://")
	input = strings.TrimSuffix(input, "/")

	if idx := strings.Index(input, "/"); idx != -1 {
		input = input[:idx]
	}

	input = strings.TrimSuffix(input, provider.hostSuffix())
	return strings.ToLower(input)
}
