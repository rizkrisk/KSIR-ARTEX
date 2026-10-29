package proxygraph

// MimeticProfile holds headers and a User-Agent that mimic a known SaaS service,
// making egress traffic blend into legitimate cloud-to-cloud communication.
type MimeticProfile struct {
	Domain    string
	UserAgent string
	Headers   map[string]string
}

// NewMimeticProfile returns a MimeticProfile for the requested style.
// Supported styles: "m365", "google", "aws", "generic".
func NewMimeticProfile(style string) MimeticProfile {
	switch style {
	case "m365":
		return MimeticProfile{
			Domain:    "outlook.office365.com",
			UserAgent: "Microsoft Office/16.0 (Windows NT 10.0; Microsoft Outlook 16.0.17628; Pro)",
			Headers: map[string]string{
				"Accept":          "application/json, text/plain, */*",
				"Accept-Language": "en-US,en;q=0.9",
				"X-Client-SKU":    "ID_NET472",
				"X-Client-Ver":    "6.38.0.0",
			},
		}
	case "google":
		return MimeticProfile{
			Domain:    "accounts.google.com",
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36",
			Headers: map[string]string{
				"Accept":          "application/json",
				"Accept-Language": "en-US,en;q=0.9",
				"Origin":          "https://accounts.google.com",
				"Sec-Fetch-Site":  "same-origin",
				"Sec-Fetch-Mode":  "cors",
			},
		}
	case "aws":
		return MimeticProfile{
			Domain:    "s3.amazonaws.com",
			UserAgent: "aws-sdk-go/1.55.7 (go1.26.3; linux; amd64)",
			Headers: map[string]string{
				"Accept":       "application/xml",
				"Content-Type": "application/x-www-form-urlencoded",
			},
		}
	default: // "generic"
		return MimeticProfile{
			Domain:    "",
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36",
			Headers: map[string]string{
				"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
				"Accept-Language": "en-US,en;q=0.5",
				"Connection":      "keep-alive",
			},
		}
	}
}
