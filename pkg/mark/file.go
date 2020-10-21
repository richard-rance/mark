package mark

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/kovetskiy/mark/pkg/log"
)

func RecurseDir(basePath string, root *Meta) (*Meta, error) {
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
			self.UpdateTitleFromPath()
			match := parent.ChildByTitle(self.Title)
			if match != nil {
				self = match
			} else {
				parent.Children = append(parent.Children, self)
			}
			recurseDir(basePath, filepath.Join(path, f.Name()), self)
		} else if strings.HasSuffix(f.Name(), ".md") {
			page := NewMeta(basePath, filepath.Join(path, f.Name()), parent)
			parent.Children = append(parent.Children, page)
		}
	}
	return nil
}
