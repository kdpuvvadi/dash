package apps

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
)

type Creds map[string]string

type Result struct {
	Online   bool
	NeedsKey bool
	Fields   map[string]string
	Error    string
}

func (d *Def) MissingCreds(creds Creds) []string {
	var missing []string
	for _, c := range d.Credentials {
		if !c.Required {
			continue
		}
		if strings.TrimSpace(creds[c.Name]) == "" {
			missing = append(missing, c.Name)
		}
	}
	return missing
}

func (d *Def) CollectMetrics(baseURL string, creds Creds) Result {
	res := Result{Fields: map[string]string{}}
	if miss := d.MissingCreds(creds); len(miss) > 0 {
		res.NeedsKey = true
		res.Error = "needs an API key — add it in app.yaml or secrets (the web UI is read-only)"
		return res
	}
	client := &http.Client{Timeout: 12 * time.Second}
	base := strings.TrimRight(baseURL, "/")

	if d.Probe != nil {
		ok, err := d.doCall(client, base, *d.Probe, creds)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		if !ok {
			res.Error = "probe failed"
			return res
		}
	}

	for _, field := range d.Collect {
		call := HTTPCall{
			Method: field.Method,
			Path:   field.Path,
			Body:   field.Body,
		}
		if call.Method == "" {
			call.Method = "GET"
		}
		body, status, err := d.request(client, base, call, creds)
		if err != nil {
			res.Fields[field.ID] = "—"
			continue
		}
		if status >= 400 {
			res.Fields[field.ID] = "—"
			continue
		}
		val, err := parseValue(field.Parse, field.Value, body)
		if err != nil {
			res.Fields[field.ID] = strings.TrimSpace(string(body))
		} else {
			res.Fields[field.ID] = val
		}
	}
	res.Online = true
	return res
}

func (d *Def) doCall(client *http.Client, base string, call HTTPCall, creds Creds) (bool, error) {
	body, status, err := d.request(client, base, call, creds)
	if err != nil {
		return false, err
	}
	_ = body
	ok := call.OKStatus
	if len(ok) == 0 {
		ok = []int{200}
	}
	for _, s := range ok {
		if status == s {
			return true, nil
		}
	}
	return false, fmt.Errorf("unexpected status %d", status)
}

func (d *Def) request(client *http.Client, base string, call HTTPCall, creds Creds) ([]byte, int, error) {
	method := call.Method
	if method == "" {
		method = "GET"
	}
	path := render(call.Path, creds)
	u := base + path
	var bodyReader io.Reader
	if call.Body != "" {
		bodyReader = strings.NewReader(render(call.Body, creds))
	}
	req, err := http.NewRequest(method, u, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	if call.Body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	applyAuth(req, d.Auth, creds)
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	return b, resp.StatusCode, err
}

func applyAuth(req *http.Request, auth Auth, creds Creds) {
	switch strings.ToLower(auth.Type) {
	case "bearer":
		tok := creds["key"]
		if tok == "" {
			tok = firstValue(creds)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	case "header":
		h := auth.Header
		if h == "" {
			h = "Authorization"
		}
		req.Header.Set(h, render(auth.Value, creds))
	case "query":
		q := req.URL.Query()
		key := auth.Query
		if key == "" {
			key = "key"
		}
		q.Set(key, render(auth.Value, creds))
		req.URL.RawQuery = q.Encode()
	case "basic":
		user := creds["username"]
		pass := creds["password"]
		if user == "" {
			user = creds["key"]
		}
		req.SetBasicAuth(user, pass)
	}
}

func render(tmpl string, creds Creds) string {
	out := tmpl
	for k, v := range creds {
		out = strings.ReplaceAll(out, "{{ "+k+" }}", v)
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}

func firstValue(m Creds) string {
	for _, v := range m {
		if v != "" {
			return v
		}
	}
	return ""
}

func parseValue(parse, expr string, body []byte) (string, error) {
	switch strings.ToLower(parse) {
	case "", "text":
		return strings.TrimSpace(string(body)), nil
	case "jsonpath":
		if expr == "" {
			expr = "$"
		}
		x, err := jp.ParseString(expr)
		if err != nil {
			return "", err
		}
		v, err := oj.Parse(body)
		if err != nil {
			return "", err
		}
		res := x.Get(v)
		if len(res) == 0 {
			return "", fmt.Errorf("no match")
		}
		return fmt.Sprint(res[0]), nil
	case "regex":
		re, err := regexp.Compile(expr)
		if err != nil {
			return "", err
		}
		m := re.FindSubmatch(body)
		if len(m) < 2 {
			if len(m) == 1 {
				return string(m[0]), nil
			}
			return "", fmt.Errorf("no match")
		}
		return string(m[1]), nil
	case "json":
		var buf bytes.Buffer
		if err := json.Compact(&buf, body); err != nil {
			return strings.TrimSpace(string(body)), nil
		}
		return buf.String(), nil
	default:
		return strings.TrimSpace(string(body)), nil
	}
}

func JoinURL(base, path string) string {
	u, err := url.Parse(base)
	if err != nil {
		return strings.TrimRight(base, "/") + path
	}
	ref, err := url.Parse(path)
	if err != nil {
		return strings.TrimRight(base, "/") + path
	}
	return u.ResolveReference(ref).String()
}
