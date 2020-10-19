package mark

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kovetskiy/mark/pkg/log"
)

func ListFiles(path string, modifiedLast time.Duration) ([]Meta, error) {
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		return nil, fmt.Errorf("error opening file %s", err)
	}

	files := make([]Meta, 0, 1)

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("error reading file meta %s", err)
	}
	if stat.IsDir() {
		var now = time.Now()
		err := filepath.Walk(path,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if strings.HasSuffix(path, ".md") {

					// Only include this file if it was modified m.Since minutes ago
					if modifiedLast != 0 {
						if info.ModTime().Unix() < now.Add(-1*modifiedLast).Unix() {
							log.Debug("skipping %s: last modified %s\n", info.Name(), info.ModTime())
							return nil
						}
					}

					files = append(files, Meta{
						FilePath: path,
					})

				}
				return nil
			})
		if err != nil {
			return nil, fmt.Errorf("unable to walk path: %s", path)
		}
	} else {
		files = append(files, Meta{
			FilePath: path,
		})
	}

	return files, nil
}
