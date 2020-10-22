package mark

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kovetskiy/mark/pkg/confluence"
	"github.com/kovetskiy/mark/pkg/log"
)

func CreateEmptyPage(
	api *confluence.API,
	spaceKey string,
	meta *Meta,
) (*confluence.PageInfo, error) {
	parentID := ""
	if meta.Parent != nil {
		parentID = meta.Parent.PageID
	}
	pageTitle := meta.Title
	nextParent := meta.Parent
	var err error
	for attempt := 0; attempt < 10; attempt++ {

		page, err := api.CreatePage(spaceKey, parentID, pageTitle, ``, meta.RelativePath)
		if err == nil {
			meta.PageID = page.ID
			return page, nil
		}
		if nextParent != nil {
			pageTitle = fmt.Sprintf("%v > %v", pageTitle, nextParent.Title)
			nextParent = nextParent.Parent
		} else {
			pageTitle = fmt.Sprintf("%v > %v", meta.Title, rand.Int())
		}
	}
	return nil, err
}

func CompilePageLinks(markdown []byte, meta *Meta, baseURL string, root *Meta) ([]byte, error) {

	linkPattern := regexp.MustCompile(`([^!]\[[^\[\]]+\])\(([^\s:?#\)]+)([#\s][^\)]*)?\)`)
	markdown = linkPattern.ReplaceAllFunc(markdown, func(link []byte) []byte {
		matches := linkPattern.FindSubmatch(link)
		if len(matches) >= 3 {
			linkText := string(matches[1])
			path := string(matches[2])
			hashAndTitle := string(matches[3])

			currentDir := meta.RelativePath
			if filepath.Ext(currentDir) != "" {
				currentDir = filepath.Dir(currentDir)
			}
			fullLinkPath := filepath.Join(currentDir, path)

			var linkedMeta *Meta
			root.Walk(func(m *Meta) error {
				if strings.EqualFold(m.RelativePath, fullLinkPath) ||
					strings.EqualFold(filepath.Join(m.RelativePath, "readme.md"), fullLinkPath) {
					linkedMeta = m
				}
				return nil
			})
			if linkedMeta == nil {
				//TODO Should we add a config option to link directly to the non-markdown file in github/bitbucket?
				log.Error(fmt.Sprintf("Link to page that is not being loaded in: %v to: %v", meta.RelativePath, path))
				return link
			}
			if linkedMeta.PageID == "" {
				// TODO We should create all of the placeholder pages before starting to process any of the bodies so we don't get into this case
				log.Warning(fmt.Sprintf("Link to parent page that has not yet been loaded. Will auto fix on next import: %v -> %v", meta.RelativePath, path))
				return link
			}

			href := fmt.Sprintf("%v/spaces/%v/pages/%v/%v", baseURL, linkedMeta.Space, linkedMeta.PageID, linkedMeta.Title)
			log.Debug(fmt.Sprintf("Link successfully replaced in: %v from:%v to: %v", meta.RelativePath, path, href))
			link = []byte(fmt.Sprintf("%v(%v%v)", linkText, href, hashAndTitle))

		}

		return link
	})

	return markdown, nil
}
