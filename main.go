package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/kovetskiy/mark/pkg/confluence"
	"github.com/kovetskiy/mark/pkg/log"
	"github.com/kovetskiy/mark/pkg/mark"
	"github.com/kovetskiy/mark/pkg/mark/includes"
	"github.com/kovetskiy/mark/pkg/mark/macro"
	"github.com/kovetskiy/mark/pkg/mark/stdlib"
	"github.com/reconquest/karma-go"
)

const (
	usage = `mark - tool for updating Atlassian Confluence pages from markdown.

This is very usable if you store documentation to your orthodox software in git
repository and don't want to do a handjob with updating Confluence page using
fucking tinymce wysiwyg enterprise core editor.

You can store a user credentials in the configuration file, which should be
located in ~/.config/mark with following format:
  username = "smith"
  password = "matrixishere"
  base_url = "http://confluence.local"
where 'smith' it's your username, 'matrixishere' it's your password and
'http://confluence.local' is base URL for your Confluence instance.

Mark understands extended file format, which, still being valid markdown,
contains several metadata headers, which can be used to locate page inside
Confluence instance and update it accordingly.

File in extended format should follow specification:

  <!-- Space: <space key> -->
  <!-- Parent: <parent 1> -->
  <!-- Parent: <parent 2> -->
  <!-- Title: <title> -->

  <page contents>

There can be any number of 'Parent' headers, if mark can't find specified
parent by title, it will be created.

Also, optional following headers are supported:

  * <!-- Layout: (article|plain) -->

    - (default) article: content will be put in narrow column for ease of
      reading;
    - plain: content will fill all page;

Mark supports Go templates, which can be included into article by using path
to the template relative to current working dir, e.g.:

  <!-- Include: <path> -->

Templates may accept configuration data in YAML format which immediately
follows include tag:

  <!-- Include: <path>
       <yaml-data> -->

Mark also supports macro definitions, which are defined as regexps which will
be replaced with specified template:

  <!-- Macro: <regexp>
       Template: <path>
       <yaml-data> -->

Capture groups can be defined in the macro's <regexp> which can be later
referenced in the <yaml-data> using ${<number>} syntax, where <number> is
number of a capture group in regexp (${0} is used for entire regexp match), for
example:

  <!-- Macro: MYJIRA-\d+
       Template: ac:jira:ticket
       Ticket: ${0} -->

By default, mark provides several built-in templates and macros:

* template 'ac:status' to include badge-like text, which accepts following
  parameters:
  - Title: text to display in the badge
  - Color: color to use as background/border for badge
    - Grey
    - Yellow
    - Red
    - Blue
  - Subtle: specify to fill badge with background or not
    - true
    - false

  See: https://confluence.atlassian.com/conf59/status-macro-792499207.html

* template 'ac:jira:ticket' to include JIRA ticket link. Parameters:
  - Ticket: Jira ticket number like BUGS-123.

* macro '@{...}' to mention user by name specified in the braces.

Usage:
  mark [options] [-u <username>] [-p <token>] [-k] [-l <url>] -f <file>
  mark [options] [-u <username>] [-p <password>] [-k] [-b <url>] -f <file>
  mark [options] [-u <username>] [-p <password>] [-k] [-n] -c <file>
  mark -v | --version
  mark -h | --help

Options:
  -u <username>         Use specified username for updating Confluence page.
  -p <token>            Use specified token for updating Confluence page.
  -r <url>              Specify the base Confluence page to update.
  -l <url>              Edit specified Confluence page.
                         If -l is not specified, file should contain metadata (see
                         above).
  -b --base-url <url>   Base URL for Confluence.
                         Alternative option for base_url config field.
  -f <file>             Use specified markdown file for converting to html.
  -k                    Lock page editing to current user only to prevent accidental
                         manual edits over Confluence Web UI.
  -s --since <minutes>  When file is a directory only process files modified in the last x minutes
  --dry-run             Resolve page and ancestry, show resulting HTML and exit.
  --compile-only        Show resulting HTML and don't update Confluence page content.
  --debug               Enable debug logs.
  --trace               Enable trace logs.
  -h --help             Show this screen and call 911.
  -v --version          Show version.
`
)

func main() {
	args, err := docopt.Parse(usage, nil, true, "3.2", false)
	if err != nil {
		panic(err)
	}

	var (
		targetFile, _ = args["-f"].(string)
		compileOnly   = args["--compile-only"].(bool)
		dryRun        = args["--dry-run"].(bool)
		editLock      = args["-k"].(bool)
	)
	since, err := argInt(args, "--since")
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	modifiedSince := time.Duration(since) * time.Minute

	log.Init(args["--debug"].(bool), args["--trace"].(bool))

	config, err := LoadConfig(filepath.Join(os.Getenv("HOME"), ".config/mark"))
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	creds, err := GetCredentials(args, config)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	api := confluence.NewAPI(creds.BaseURL, creds.Username, creds.Password)

	spaceKey := ""
	var fileMetadata []*mark.Meta
	if creds.RootPageID != "" {
		// Importing a directory
		rootFile, err := api.GetPageByID(creds.RootPageID)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		spaceKey = rootFile.Space.Key

		existingPages, err := api.ListChildPages(rootFile.ID)

		fileMetadata, err = mark.ListFiles(targetFile, modifiedSince)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		err = mark.PairPages(existingPages, fileMetadata)
	} else {
		//Importing a single file
		fileMetadata = []*mark.Meta{
			&mark.Meta{
				//TODO build it in a backwards compatible way
			},
		}
	}

	for _, meta := range fileMetadata {

		markdown, err := ioutil.ReadFile(meta.FilePath)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		markdown, err = meta.UpdateFromHeader(markdown)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		if meta.Title == "" {
			err = meta.UpdateTitleFromBody(markdown, 10)
			if err != nil {
				log.Fatal(err)
			}
		}

		if meta.Title == "" {
			meta.Title = strings.TrimSuffix(filepath.Base(meta.FilePath), filepath.Ext(meta.FilePath))
		}

		if meta.Space == "" {
			meta.Space = spaceKey
		}

		err = meta.UpdateAttachmentsFromBody(markdown)
		if err != nil {
			log.Fatal(err)
		}

		err = meta.Validate()
		if err != nil {
			log.Fatal(err)
		}

		stdlib, err := stdlib.New(api)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		templates := stdlib.Templates

		var recurse bool

		for {
			templates, markdown, recurse, err = includes.ProcessIncludes(
				markdown,
				templates,
			)
			if err != nil {
				log.Fatal(err)
				os.Exit(1)
			}

			if !recurse {
				break
			}
		}

		macros, markdown, err := macro.ExtractMacros(markdown, templates)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		macros = append(macros, stdlib.Macros...)

		for _, macro := range macros {
			markdown, err = macro.Apply(markdown)
			if err != nil {
				log.Fatal(err)
				os.Exit(1)
			}
		}

		if dryRun {
			compileOnly = true

			_, _, err := mark.ResolvePage(dryRun, api, meta)
			if err != nil {
				log.Fatalf(err, "unable to resolve page location")
				os.Exit(1)
			}
		}

		if compileOnly {
			fmt.Println(mark.CompileMarkdown(markdown, stdlib))
			os.Exit(0)
		}

		var target *confluence.PageInfo

		if meta.PageID != "" {
			parent, page, err := mark.ResolvePage(dryRun, api, meta)
			if err != nil {
				log.Fatalf(
					karma.Describe("title", meta.Title).Reason(err),
					"unable to resolve page",
				)
			}

			if page == nil {
				page, err = api.CreatePage(meta.Space, parent, meta.Title, ``)
				if err != nil {
					log.Fatalf(
						err,
						"can't create page %q",
						meta.Title,
					)
				}
			}
			meta.PageID = page.ID
			target = page
		} else {

			page, err := api.GetPageByID(creds.PageID)
			if err != nil {
				log.Fatalf(err, "unable to retrieve page by id")
			}

			target = page
		}

		fileDir := filepath.Dir(meta.FilePath)

		attaches, err := mark.ResolveAttachments(api, target, fileDir, meta.Attachments)
		if err != nil {
			log.Fatalf(err, "unable to create/update attachments")
		}

		markdown = mark.CompileAttachmentLinks(markdown, attaches)

		html := mark.CompileMarkdown(markdown, stdlib)

		{
			var buffer bytes.Buffer

			err := stdlib.Templates.ExecuteTemplate(
				&buffer,
				"ac:layout",
				struct {
					Layout string
					Body   string
				}{
					Layout: meta.Layout,
					Body:   html,
				},
			)
			if err != nil {
				log.Fatal(err)
			}

			html = buffer.String()
		}

		err = api.UpdatePage(target, html)
		if err != nil {
			log.Fatal(err)
		}

		if editLock {
			log.Infof(
				nil,
				`edit locked on page %q by user %q to prevent manual edits`,
				target.Title,
				creds.Username,
			)

			err := api.RestrictPageUpdates(
				target,
				creds.Username,
			)
			if err != nil {
				log.Fatal(err)
			}
		}

		log.Infof(
			nil,
			"page successfully updated: %s",
			creds.BaseURL+target.Links.Full,
		)

		fmt.Println(
			creds.BaseURL + target.Links.Full,
		)
	}
}

func argInt(args map[string]interface{}, key string) (int, error) {
	arg, ok := args[key].(string)
	if !ok {
		return 0, nil
	}
	v, err := strconv.Atoi(arg)
	if err != nil {
		return 0, fmt.Errorf("%v is not a valid int: %w", key, err)
	}
	return v, nil
}
