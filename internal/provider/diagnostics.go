package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	hexcore "github.com/2chevskii/go-hexcore"
)

// Never forward err.Error(), API messages, URLs, headers, bodies or action messages:
// even SDK error text can contain arbitrary remote user data or secrets.
func safeError(err error) string {
	if errors.Is(err, context.Canceled) {
		return "The operation was canceled."
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "The operation deadline was exceeded."
	}
	var apiErr *hexcore.APIError
	if errors.As(err, &apiErr) {
		return fmt.Sprintf("HexCore returned HTTP %d (%s).", apiErr.StatusCode, http.StatusText(apiErr.StatusCode))
	}
	var actionErr *hexcore.ActionError
	if errors.As(err, &actionErr) {
		return "The HexCore action failed. Inspect its status in the control panel."
	}
	var catalogErr *catalogError
	if errors.As(err, &catalogErr) {
		return catalogErr.message
	}
	return "The SDK request or response could not be completed. Check connectivity and the control panel."
}
