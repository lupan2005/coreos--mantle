// Copyright 2016 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package index

import (
	"bytes"
	"html/template"
	"net/url"
	"path"
	"strings"

	"github.com/coreos/mantle/Godeps/_workspace/src/golang.org/x/net/context"
	gs "github.com/coreos/mantle/Godeps/_workspace/src/google.golang.org/api/storage/v1"

	"github.com/coreos/mantle/lang/maps"
	"github.com/coreos/mantle/storage"
)

var (
	indexTemplate *template.Template
)

const (
	indexText = `<html>
    <head>
	<title>{{.Title}}</title>
	<meta http-equiv="X-Clacks-Overhead" content="GNU Terry Pratchett" />
    </head>
    <body>
    <h1>{{.Title}}</h1>
    {{range .SubDirs}}
	[dir] <a href="{{.Link}}/">{{.Title}}</a> </br>
    {{end}}
    {{range .Objects}}
	[file] <a href="{{.Link}}">{{.Title}}</a> </br>
    {{end}}
    </body>
</html>
`
)

func init() {
	indexTemplate = template.Must(template.New("index").Parse(indexText))
}

type linkParams struct {
	Title string
	Link  string
}

type indexParams struct {
	Title   string
	SubDirs []linkParams
	Objects []linkParams
}

type Indexer struct {
	bucket  *storage.Bucket
	prefix  string
	Title   string
	SubDirs map[string]string
	Objects map[string]*gs.Object
}

func (t *IndexTree) Indexer(dir string) *Indexer {
	return &Indexer{
		bucket:  t.bucket,
		prefix:  dir,
		SubDirs: t.SubDirs(dir),
		Objects: t.Objects(dir),
	}
}

func (i *Indexer) DeleteRedirect(ctx context.Context) error {
	if i.prefix == "" {
		return nil
	}
	return i.bucket.Delete(ctx, strings.TrimSuffix(i.prefix, "/"))
}

func (i *Indexer) DeleteDirectory(ctx context.Context) error {
	if i.prefix == "" {
		return nil
	}
	return i.bucket.Delete(ctx, i.prefix)
}

func (i *Indexer) DeleteIndexHTML(ctx context.Context) error {
	return i.bucket.Delete(ctx, i.prefix+"index.html")
}

func (i *Indexer) UpdateRedirect(ctx context.Context) error {
	if i.prefix == "" {
		return nil
	}

	name := strings.TrimSuffix(i.prefix, "/")
	obj := gs.Object{
		Name:         name,
		ContentType:  "text/html",
		CacheControl: "public, max-age=60",
	}

	link := escapePath(path.Base(name))
	buf := bytes.NewBuffer(make([]byte, 0, 256))
	buf.WriteString("<html><head>\n")
	// TODO: include <link rel="canonical" href="d.Prefix"/>
	// I suspect that's only meaningful if we switch to absolute paths
	buf.WriteString(`<meta http-equiv="refresh" content="0;url=`)
	buf.WriteString(link)
	buf.WriteString("/\">\n</head></html>\n")

	return i.bucket.Upload(ctx, &obj, bytes.NewReader(buf.Bytes()))
}

func (i *Indexer) updateHTML(ctx context.Context, suffix string) error {
	title := template.HTMLEscapeString(i.bucket.Name() + "/" + i.prefix)
	params := indexParams{
		Title:   title,
		SubDirs: sortedLinks(i.SubDirs),
		Objects: sortedLinks(i.Objects),
	}
	obj := gs.Object{
		Name:         i.prefix + suffix,
		ContentType:  "text/html",
		CacheControl: "public, max-age=60",
	}

	buf := bytes.Buffer{}
	if err := indexTemplate.Execute(&buf, &params); err != nil {
		return err
	}

	return i.bucket.Upload(ctx, &obj, bytes.NewReader(buf.Bytes()))
}

func (i *Indexer) UpdateDirectoryHTML(ctx context.Context) error {
	if i.prefix == "" {
		return nil
	}

	return i.updateHTML(ctx, "")
}

func (i *Indexer) UpdateIndexHTML(ctx context.Context) error {
	return i.updateHTML(ctx, "index.html")
}

func sortedLinks(m interface{}) (links []linkParams) {
	for _, name := range maps.NaturalKeys(m) {
		links = append(links, linkParams{
			Title: template.HTMLEscapeString(name),
			Link:  escapePath(name),
		})
	}
	return
}

func escapePath(path string) string {
	u := url.URL{Path: path}
	return u.EscapedPath()
}
