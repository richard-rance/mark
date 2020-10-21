package mark

import (
	"fmt"
	"math/rand"

	"github.com/kovetskiy/mark/pkg/confluence"
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
