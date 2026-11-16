package digest

import (
	"bufio"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

// robotsRule is one Allow/Disallow line (RFC 9309).
type robotsRule struct {
	allow  bool
	length int // pattern length, for longest-match precedence
	re     *regexp.Regexp
}

// robotsRules are the rules of the group that applies to skim.
type robotsRules []robotsRule

// allows applies RFC 9309 matching to path+query: the longest matching
// pattern wins, Allow wins ties, no match means allowed.
func (rs robotsRules) allows(u *url.URL) bool {
	target := u.EscapedPath()
	if target == "" {
		target = "/"
	}
	if u.RawQuery != "" {
		target += "?" + u.RawQuery
	}
	best, allowed := -1, true
	for _, r := range rs {
		if !r.re.MatchString(target) {
			continue
		}
		if r.length > best || (r.length == best && r.allow) {
			best, allowed = r.length, r.allow
		}
	}
	return allowed
}

// robotsCache fetches robots.txt once per host. A missing or failing
// robots.txt allows everything.
type robotsCache struct {
	client *http.Client
	mu     sync.Mutex
	hosts  map[string]*robotsEntry
}

type robotsEntry struct {
	once  sync.Once
	rules robotsRules
}

func newRobotsCache(c *http.Client) *robotsCache {
	return &robotsCache{client: c, hosts: map[string]*robotsEntry{}}
}

func (c *robotsCache) allowed(u *url.URL) bool {
	key := u.Scheme + "://" + u.Host
	c.mu.Lock()
	e := c.hosts[key]
	if e == nil {
		e = &robotsEntry{}
		c.hosts[key] = e
	}
	c.mu.Unlock()
	e.once.Do(func() { e.rules = c.fetch(key) })
	return e.rules.allows(u)
}

func (c *robotsCache) fetch(base string) robotsRules {
	req, err := http.NewRequest("GET", base+"/robots.txt", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	return parseRobots(io.LimitReader(resp.Body, 512<<10))
}

// parseRobots returns the rules for the "skim-fetch" group, else "*".
func parseRobots(r io.Reader) robotsRules {
	groups := map[string]robotsRules{}
	var agents []string
	inRules := false
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k, v = strings.ToLower(strings.TrimSpace(k)), strings.TrimSpace(v)
		switch k {
		case "user-agent":
			if inRules {
				agents, inRules = nil, false
			}
			a := strings.ToLower(v)
			agents = append(agents, a)
			if _, ok := groups[a]; !ok {
				groups[a] = robotsRules{}
			}
		case "allow", "disallow":
			inRules = true
			if v == "" {
				continue
			}
			rule := robotsRule{allow: k == "allow", length: len(v), re: robotsPattern(v)}
			for _, a := range agents {
				groups[a] = append(groups[a], rule)
			}
		default:
			inRules = true
		}
	}
	if g, ok := groups["skim-fetch"]; ok {
		return g
	}
	return groups["*"]
}

// robotsPattern converts a robots.txt path pattern (* wildcard, trailing $
// anchor) into a prefix regexp.
func robotsPattern(p string) *regexp.Regexp {
	anchored := strings.HasSuffix(p, "$")
	p = strings.TrimSuffix(p, "$")
	parts := strings.Split(p, "*")
	for i := range parts {
		parts[i] = regexp.QuoteMeta(parts[i])
	}
	expr := "^" + strings.Join(parts, ".*")
	if anchored {
		expr += "$"
	}
	return regexp.MustCompile(expr)
}
