package mark

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kovetskiy/mark/pkg/log"
)

const (
	HeaderParent     = `Parent`
	HeaderSpace      = `Space`
	HeaderTitle      = `Title`
	HeaderLayout     = `Layout`
	HeaderAttachment = `Attachment`
)

type Meta struct {
	FilePath    string
	Parents     []string
	Space       string
	Title       string
	Layout      string
	Attachments map[string]string
}

var (
	reHeaderPatternV1 = regexp.MustCompile(`\[\]:\s*#\s*\(([^:]+):\s*(.*)\)`)
	reHeaderPatternV2 = regexp.MustCompile(`<!--\s*([^:]+):\s*(.*)\s*-->`)
	//titlePattern      = regexp.MustCompile(`^#\s.*$`)
)

func NewMeta(filePath string) *Meta {
	meta := &Meta{
		FilePath: filePath,
	}
	meta.UpdateParentsFromPath()

	return meta
}

func (m *Meta) UpdateFromHeader(data []byte) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("No metadata object provided.")
	}
	var offset int

	scanner := bufio.NewScanner(bytes.NewBuffer(data))
	for scanner.Scan() {
		line := scanner.Text()

		if err := scanner.Err(); err != nil {
			return nil, err
		}

		offset += len(line) + 1

		matches := reHeaderPatternV2.FindStringSubmatch(line)
		if matches == nil {
			matches = reHeaderPatternV1.FindStringSubmatch(line)
			if matches == nil {
				break
			}

			log.Warningf(
				fmt.Errorf(`legacy header usage found: %s`, line),
				"please use new header format: <!-- %s: %s -->",
				matches[1],
				matches[2],
			)
		}

		m.Attachments = make(map[string]string)

		header := strings.Title(matches[1])

		var value string
		if len(matches) > 1 {
			value = strings.TrimSpace(matches[2])
		}

		switch header {
		case HeaderParent:
			m.Parents = append(m.Parents, value)

		case HeaderSpace:
			m.Space = strings.TrimSpace(value)

		case HeaderTitle:
			m.Title = strings.TrimSpace(value)

		case HeaderLayout:
			m.Layout = strings.TrimSpace(value)

		case HeaderAttachment:
			m.Attachments[value] = value

		default:
			log.Errorf(
				nil,
				`encountered unknown header %q line: %#v`,
				header,
				line,
			)

			continue
		}
	}

	if m.Space == "" {
		return nil, fmt.Errorf(
			"space key is not set (%s header is not set)",
			HeaderSpace,
		)
	}

	if m.Title == "" {
		return nil, fmt.Errorf(
			"page title is not set (%s header is not set)",
			HeaderTitle,
		)
	}

	return data[offset:], nil
}

func (m *Meta) UpdateParentsFromPath() {
	dirs := filepath.SplitList(m.FilePath)
	parents := make([]string, 0)
	for _, dir := range dirs {
		parents = append(parents, strings.Trim(dir, string(os.PathSeparator)))
	}
	m.Parents = parents
}
