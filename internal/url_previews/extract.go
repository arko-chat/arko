package urlpreviews

import "regexp"

var urlRegex = regexp.MustCompile(`https?://[^\s<>"{}|\\^\[\]` + "`" + `]+`)

func ExtractURLs(text string) []string {
	seen := make(map[string]struct{})
	var urls []string
	for _, u := range urlRegex.FindAllString(text, -1) {
		if _, ok := seen[u]; !ok {
			seen[u] = struct{}{}
			urls = append(urls, u)
		}
	}
	return urls
}

func HasURLs(text string) bool {
	return len(ExtractURLs(text)) > 0
}
