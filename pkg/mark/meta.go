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
	reHeaderPatternV1     = regexp.MustCompile(`\[\]:\s*#\s*\(([^:]+):\s*(.*)\)`)
	reHeaderPatternV2     = regexp.MustCompile(`<!--\s*([^:]+):\s*(.*)\s*-->`)
	titlePattern          = regexp.MustCompile(`^#\s(.*)$`)
	fileAttachmentPattern = regexp.MustCompile(`!\[([^\[\]]+)\]\(([^\s\)]+)(\s+\"[^\"]+\")?\)`)
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

	return data[offset:], nil
}

func (m *Meta) UpdateParentsFromPath() {
	var file string
	path := filepath.Dir(m.FilePath)
	parents := make([]string, 0)
	for path != "" {
		path, file = filepath.Split(path)
		path = strings.TrimRight(path, string(os.PathSeparator))
		file = strings.ReplaceAll(file, ".", " ")
		file = strings.ReplaceAll(file, "-", " ")
		file = strings.ReplaceAll(file, "_", " ")
		file = strings.Trim(file, " "+string(os.PathSeparator))
		if file != "" {
			parents = append([]string{strings.Title(file)}, parents...)
		}
	}
	m.Parents = parents
}

func (m *Meta) UpdateTitleFromBody(data []byte, limit int) error {
	lineIndex := 0
	scanner := bufio.NewScanner(bytes.NewBuffer(data))
	for scanner.Scan() {
		line := scanner.Text()

		if err := scanner.Err(); err != nil {
			return err
		}

		matches := titlePattern.FindStringSubmatch(line)
		if matches != nil && len(matches) > 1 {
			m.Title = matches[1]
			return nil
		}

		lineIndex++
		if lineIndex > limit {
			return nil
		}
	}

	return nil
}

func (m *Meta) UpdateAttachmentsFromBody(data []byte) error {
	scanner := bufio.NewScanner(bytes.NewBuffer(data))
	for scanner.Scan() {
		line := scanner.Text()

		if err := scanner.Err(); err != nil {
			return err
		}

		matches := fileAttachmentPattern.FindStringSubmatch(line)
		if matches != nil && len(matches) > 2 {
			m.Attachments[matches[2]] = matches[2]
		}
	}

	return nil
}

func (m *Meta) Validate() error {
	if m.FilePath == "" {
		return fmt.Errorf("file path is not set")
	}

	if m.Space == "" {
		return fmt.Errorf("target space is not set (%s header or option is not set)",
			HeaderSpace,
		)
	}

	if m.Title == "" {
		return fmt.Errorf(
			"page title is not set (%s header is not set or could not be inferred)",
			HeaderTitle,
		)
	}

	return nil
}
