package actions

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"unicode"

	backendservice "something/backend/service"
)

const maximumExternalURLBytes = 8192

// ExternalURLLauncher is the narrow native capability used to open web URLs.
type ExternalURLLauncher interface {
	OpenURL(value string) error
}

// OpenExternalURL validates an untrusted URL before sending it to the OS.
// It intentionally does not use Wails' BrowserOpenURL validator because that
// validator rejects the tilde required by Text Fragment URLs.
func OpenExternalURL(ctx context.Context, launcher ExternalURLLauncher, value string) error {
	if launcher == nil {
		return errors.New("opening web pages is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maximumExternalURLBytes {
		return &backendservice.ValidationError{Field: "url", Message: "This result contains an invalid URL."}
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return &backendservice.ValidationError{Field: "url", Message: "This result contains an invalid URL."}
		}
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return &backendservice.ValidationError{Field: "url", Message: "This result contains an invalid URL."}
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return &backendservice.ValidationError{Field: "url", Message: "Only HTTP and HTTPS results can be opened."}
	}
	if err := launcher.OpenURL(parsed.String()); err != nil {
		return errors.New("the default browser could not open this result")
	}
	return nil
}
