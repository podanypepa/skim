package triage

import "github.com/podanypepa/skim/internal/store"

// dayArticle is an archived article with the day of its file.
type dayArticle struct {
	Day string
	store.Article
}

// loadArchive returns all archived articles, oldest day first, each id
// only once (its first occurrence; later duplicates come from a crashed
// fetch run).
func loadArchive(dataDir string) ([]dayArticle, error) {
	days, err := store.Days(dataDir, "articles")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []dayArticle
	for _, d := range days {
		arts, err := store.ReadArticles(dataDir, d)
		if err != nil {
			return nil, err
		}
		for _, a := range arts {
			if seen[a.ID] {
				continue
			}
			seen[a.ID] = true
			out = append(out, dayArticle{Day: d, Article: a})
		}
	}
	return out, nil
}
