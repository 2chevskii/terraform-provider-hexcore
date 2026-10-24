package provider

import (
	"context"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func validToken(s string) bool {
	return s != "" && !strings.ContainsFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) && !strings.EqualFold(s, "Bearer")
}
func validEndpoint(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u != nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.Opaque == "" && u.User == nil && u.RawQuery == "" && !u.ForceQuery && u.Fragment == ""
}

type tokenValidator struct{}

func (tokenValidator) Description(context.Context) string {
	return "A nonempty raw token without whitespace or a Bearer prefix."
}
func (v tokenValidator) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }
func (tokenValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if !req.ConfigValue.IsNull() && !req.ConfigValue.IsUnknown() && !validToken(req.ConfigValue.ValueString()) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid API token", "Use a nonempty raw token without whitespace or a Bearer prefix.")
	}
}

type endpointValidator struct{}

func (endpointValidator) Description(context.Context) string {
	return "An absolute HTTP(S) URL without credentials, query, or fragment."
}
func (v endpointValidator) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }
func (endpointValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if !req.ConfigValue.IsNull() && !req.ConfigValue.IsUnknown() && !validEndpoint(req.ConfigValue.ValueString()) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid API endpoint", "Use an absolute HTTP(S) URL without credentials, query, or fragment.")
	}
}

var idPattern = regexp.MustCompile(`^[0-9]*[1-9][0-9]*$`)
var slugPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// IDs remain strings, including values larger than a machine integer.
type idValidator struct{}

func (idValidator) Description(context.Context) string {
	return "A positive decimal resource ID represented as a string."
}
func (v idValidator) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }
func (idValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if !req.ConfigValue.IsNull() && !req.ConfigValue.IsUnknown() && !idPattern.MatchString(req.ConfigValue.ValueString()) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid VDS ID", "Use a positive decimal ID string.")
	}
}

type nameValidator struct{}

func (nameValidator) Description(context.Context) string {
	return "A nonblank name of at most 45 UTF-8 bytes (conservative provider policy)."
}
func (v nameValidator) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }
func (nameValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	s := req.ConfigValue.ValueString()
	if strings.TrimSpace(s) == "" || len(s) > 45 || strings.ContainsFunc(s, unicode.IsControl) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid VDS name", "Use a nonblank name of at most 45 UTF-8 bytes without control characters. This conservative provider policy avoids observed API truncation; Unicode API boundaries are unverified.")
	}
}
