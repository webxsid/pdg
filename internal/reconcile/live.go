package reconcile

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/webxsid/pdg/internal/state"
	"golang.org/x/net/html"
)

const (
	maxLiveBody       = 4 << 20
	maxLiveRedirects  = 10
	liveVerifyTimeout = 20 * time.Second
)

type LiveProgress func(resource, rawURL string)

// VerifyStandardSiteLive checks deployed Standard.site artifacts without mutating local state.
func VerifyStandardSiteLive(ctx context.Context, project Project, client *http.Client) []Finding {
	return VerifyStandardSiteLiveWithProgress(ctx, project, client, nil)
}

func VerifyStandardSiteLiveWithProgress(ctx context.Context, project Project, client *http.Client, progress LiveProgress) []Finding {
	if client == nil {
		client = &http.Client{Timeout: liveVerifyTimeout}
	}
	findings := make([]Finding, 0)
	standard := project.Config.Integrations.StandardSite
	if !standard.Enabled {
		return []Finding{{Integration: standardSite, Resource: "configuration", Status: Failure, Message: "Standard.site is not configured for this project."}}
	}
	publicationState, ok := project.State.Integrations[standardSite]
	if !ok || publicationState.Publication == nil || publicationState.Publication.URI == "" {
		return []Finding{{Integration: standardSite, Resource: "publication state", Status: Conflict, Message: "No provisioned Standard.site publication exists in .pdg/state.json."}}
	}
	publication := publicationState.Publication.URI
	if err := ValidatePublicationURI(publication, standard.Identity); err != nil {
		return []Finding{{Integration: standardSite, Resource: "publication identity", Status: Conflict, Message: err.Error()}}
	}
	publicationURL, err := deployedArtifactURL(project.Config.Site.URL)
	if err != nil {
		return []Finding{{Integration: standardSite, Resource: "publication", Status: Failure, Message: err.Error()}}
	}
	if progress != nil {
		progress("publication", publicationURL)
	}
	body, status, err := getLive(ctx, client, publicationURL, maxLiveBody)
	if err != nil {
		findings = append(findings, liveHTTPFinding("publication", publicationURL, status, err))
	} else if strings.TrimSpace(string(body)) != publication {
		findings = append(findings, Finding{Integration: standardSite, Resource: "publication", Status: Drift, Message: "deployed publication discovery file does not match .pdg state", Expected: publication, Actual: strings.TrimSpace(string(body))})
	} else {
		findings = append(findings, Finding{Integration: standardSite, Resource: "publication", Status: OK, Message: "deployed publication discovery file is correct."})
	}

	for path, document := range project.State.Documents {
		if document == nil || document.CanonicalURL == "" {
			continue
		}
		target, exists := document.Targets[state.TargetStandardSite]
		if !exists || target.URI == "" {
			continue
		}
		if err := validateDocumentURI(target.URI, standard.Identity); err != nil {
			findings = append(findings, Finding{Integration: standardSite, Resource: path, Status: Conflict, Message: err.Error()})
			continue
		}
		pageURL, err := validHTTPURL(document.CanonicalURL)
		if err != nil {
			findings = append(findings, Finding{Integration: standardSite, Resource: path, Status: Failure, Message: fmt.Sprintf("invalid canonical URL: %v", err)})
			continue
		}
		if progress != nil {
			progress(path, pageURL)
		}
		body, status, err := getLive(ctx, client, pageURL, maxLiveBody)
		if err != nil {
			findings = append(findings, liveHTTPFinding(path, pageURL, status, err))
			continue
		}
		count, observed := documentLinks(body)
		switch {
		case count == 0:
			findings = append(findings, Finding{Integration: standardSite, Resource: path, Status: Drift, Message: "deployed page does not contain a Standard.site document verification link", Expected: target.URI})
		case count != 1:
			findings = append(findings, Finding{Integration: standardSite, Resource: path, Status: Drift, Message: fmt.Sprintf("deployed page contains %d Standard.site document verification links; expected one", count), Expected: target.URI, Actual: strings.Join(observed, ", ")})
		case observed[0] != target.URI:
			findings = append(findings, Finding{Integration: standardSite, Resource: path, Status: Drift, Message: "deployed page has the wrong Standard.site document URI", Expected: target.URI, Actual: observed[0]})
		default:
			findings = append(findings, Finding{Integration: standardSite, Resource: path, Status: OK, Message: "deployed document verification link is correct."})
		}
	}
	return findings
}

func deployedArtifactURL(raw string) (string, error) {
	base, err := validHTTPURL(raw)
	if err != nil {
		return "", fmt.Errorf("invalid site URL: %w", err)
	}
	parsed, _ := url.Parse(base)
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/.well-known/site.standard.publication"
	parsed.RawQuery, parsed.Fragment = "", ""
	return parsed.String(), nil
}

func validHTTPURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return "", errors.New("URL must be an HTTP(S) URL without userinfo")
	}
	return parsed.String(), nil
}

func getLive(ctx context.Context, client *http.Client, rawURL string, limit int64) ([]byte, int, error) {
	requestCtx, cancel := context.WithTimeout(ctx, liveVerifyTimeout)
	defer cancel()
	transport := *client
	transport.CheckRedirect = func(req *http.Request, redirects []*http.Request) error {
		if len(redirects)+1 > maxLiveRedirects {
			return errors.New("too many redirects")
		}
		if _, err := validHTTPURL(req.URL.String()); err != nil {
			return fmt.Errorf("unsafe redirect: %w", err)
		}
		return nil
	}
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "pdg/live-verifier")
	resp, err := transport.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("deployed site returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if int64(len(data)) > limit {
		return nil, resp.StatusCode, errors.New("response exceeds maximum supported size")
	}
	return data, resp.StatusCode, nil
}

func documentLinks(data []byte) (int, []string) {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return 0, nil
	}
	var links []string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "link" && hasRel(node, "site.standard.document") {
			links = append(links, attr(node, "href"))
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return len(links), links
}

func liveHTTPFinding(resource, rawURL string, status int, err error) Finding {
	message := fmt.Sprintf("GET %s failed: %v", rawURL, err)
	if status > 0 {
		message = fmt.Sprintf("GET %s returned HTTP %d", rawURL, status)
	}
	return Finding{Integration: standardSite, Resource: resource, Status: Failure, Message: message}
}

func validateDocumentURI(raw, did string) error {
	parts := strings.Split(strings.TrimPrefix(raw, "at://"), "/")
	if !strings.HasPrefix(raw, "at://") || len(parts) != 3 || parts[0] != did || parts[1] != "site.standard.document" || parts[2] == "" || strings.ContainsAny(raw, "?#") {
		return errors.New("invalid Standard.site document URI")
	}
	return nil
}

func hasRel(node *html.Node, wanted string) bool {
	for _, value := range strings.Fields(attr(node, "rel")) {
		if value == wanted {
			return true
		}
	}
	return false
}
func attr(node *html.Node, name string) string {
	for _, value := range node.Attr {
		if value.Key == name {
			return value.Val
		}
	}
	return ""
}
