package mark

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/kovetskiy/mark/pkg/log"
)

func NewMetaTree(basePath string, title string, pageID string) (*Meta, error) {
	root := NewMeta(basePath, basePath, nil)
	root.Directory = true
	root.Title = title
	if root.Title == "" {
		root.UpdateTitleFromPath()
	}
	root.PageID = pageID
	err := recurseDir(basePath, basePath, root)
	return root, err
}

func recurseDir(basePath, path string, parent *Meta) error {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		if f.IsDir() {
			self := NewMeta(basePath, filepath.Join(path, f.Name()), parent)
			self.Directory = true
			self.UpdateTitleFromPath()
			match := parent.ChildByTitle(self.Title)
			if match != nil {
				self = match
			} else {
				parent.Children = append(parent.Children, self)
			}
			recurseDir(basePath, filepath.Join(path, f.Name()), self)
		} else if strings.HasSuffix(f.Name(), ".md") {
			if strings.ToLower(f.Name()) == "readme.md" && parent.Directory {
				parent.FileSystemPath = filepath.Join(path, f.Name())
				parent.Directory = false
				continue
			}
			page := NewMeta(basePath, filepath.Join(path, f.Name()), parent)
			parent.Children = append(parent.Children, page)
		}
	}
	return nil
}
