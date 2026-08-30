package utils

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"errors"

	"github.com/PuerkitoBio/goquery"
	"github.com/bmaupin/go-epub"
	"github.com/gabriel-vasile/mimetype"
	"github.com/yann0917/dedao-gui/backend/request"
)

type EpubOptions struct {
	Cover       string
	Title       string
	Author      string
	Description string
	Output      string
	ImagesDir   string
	FontsDir    string
	HTML        []HtmlContent
	Verbose     bool
	PTitle      map[int]string
	Toc         []EbookToc
}

type HtmlContent struct {
	Content   string
	ChapterID string
	Toc       []EbookToc
}

type HtmlToEpub struct {
	EpubOptions
	DefaultCover   []byte
	book           *epub.Epub
	imgIdx         int
	contentCSS     string
	stylesheetTemp string
}

// contentStylesheet supplies the reading defaults the source SVG never carries:
// line spacing, paragraph rhythm, CJK-appropriate indentation and image
// fitting. Sizes stay relative so the reader's own font-size setting still
// scales everything; nothing here sets an absolute font-size or a font-family.
const contentStylesheet = `html {
  -epub-hyphens: auto;
  hyphens: auto;
}

body {
  margin: 0 5%;
  line-height: 1.75;
  text-align: justify;
  word-wrap: break-word;
  -webkit-hyphens: auto;
  hyphens: auto;
}

p {
  margin: 0 0 0.6em;
  text-indent: 2em;
  line-height: inherit;
}

/* a paragraph that only carries an image should not be indented */
p:has(> img),
p > img {
  text-indent: 0;
}

h1, h2, h3, h4, h5, h6 {
  margin: 1.4em 0 0.7em;
  line-height: 1.4;
  text-indent: 0;
  text-align: left;
  page-break-after: avoid;
  break-after: avoid;
}

div.header1, div.header2, div.header3,
div.header4, div.header5, div.header6 {
  text-indent: 0;
}

img {
  max-width: 100%;
  height: auto;
}

sup, sub {
  line-height: 1;
  font-size: 0.75em;
}

a {
  text-decoration: none;
}

aside[epub|type~="footnote"] {
  font-size: 0.9em;
  line-height: 1.6;
}

ol.duokan-footnote-content {
  text-indent: 0;
}
`

func (h *HtmlToEpub) Run() (err error) {
	if len(h.HTML) == 0 {
		return errors.New("no .html file given")
	}
	h.PTitle = make(map[int]string)
	return h.run()
}
func (h *HtmlToEpub) run() (err error) {
	err = h.genBook()
	if err != nil {
		return
	}
	defer h.cleanupStylesheet()

	for _, html := range h.HTML {
		err = h.add(html)
		if err != nil {
			err = fmt.Errorf("parse %#v failed: %s", html, err)
			return
		}
	}

	err = h.book.Write(h.Output)
	if err != nil {
		return fmt.Errorf("cannot write output epub: %s", err)
	}

	return
}

func (h *HtmlToEpub) genBook() error {
	h.book = epub.NewEpub(h.Title)
	h.book.SetAuthor(h.Author)
	h.book.SetDescription(h.Description)
	if err := h.setStylesheet(); err != nil {
		// styling is an improvement, not a precondition for a readable book
		log.Printf("警告: 无法添加正文样式表: %v", err)
	}
	return h.setCover()
}

// setStylesheet registers the reading stylesheet once so every section can
// reference it. Chapters carry no styling of their own beyond inline spans.
//
// AddCSS only records the source path — the file is not read until Write — so
// the temp file has to outlive this call and is cleaned up after the book is
// written.
func (h *HtmlToEpub) setStylesheet() error {
	temp, err := os.CreateTemp("", "dedao-content-*.css")
	if err != nil {
		return fmt.Errorf("can't create stylesheet tempfile: %w", err)
	}

	if _, err = temp.WriteString(contentStylesheet); err != nil {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
		return fmt.Errorf("can't write stylesheet: %w", err)
	}
	if err = temp.Close(); err != nil {
		_ = os.Remove(temp.Name())
		return fmt.Errorf("can't close stylesheet: %w", err)
	}
	h.stylesheetTemp = temp.Name()

	cssPath, err := h.book.AddCSS(temp.Name(), "content.css")
	if err != nil {
		return fmt.Errorf("can't add stylesheet: %w", err)
	}
	h.contentCSS = cssPath
	return nil
}

func (h *HtmlToEpub) cleanupStylesheet() {
	if h.stylesheetTemp == "" {
		return
	}
	_ = os.Remove(h.stylesheetTemp)
	h.stylesheetTemp = ""
}

func (h *HtmlToEpub) setCover() (err error) {
	if h.Cover == "" {
		temp, err := os.CreateTemp("", "html-to-epub")
		if err != nil {
			return fmt.Errorf("can't create tempfile: %s", err)
		}
		_, err = temp.Write(h.DefaultCover)
		if err != nil {
			return fmt.Errorf("can't write tempfile: %s", err)
		}
		_ = temp.Close()

		h.Cover = temp.Name()
	}

	m, err := mimetype.DetectFile(h.Cover)
	if err != nil {
		return fmt.Errorf("can't detect cover mime type %s", err)
	}
	cover, err := h.book.AddImage(h.Cover, "cover"+m.Extension())
	if err != nil {
		return fmt.Errorf("can't add cover %s", err)
	}
	h.book.SetCover(cover, "")

	return
}

func (h *HtmlToEpub) add(html HtmlContent) (err error) {
	refs := make(map[string]string)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html.Content))
	if err != nil {
		return
	}

	images, err := h.saveImages(doc)
	if err != nil {
		return err
	}
	doc.Find("img").
		Each(func(i int, img *goquery.Selection) {
			h.changeRef(html.Content, img, refs, images)
		})
	content, err := doc.Find("body").Html()
	if err != nil {
		return
	}
	if html.ChapterID != "cover.xhtml" {
		if len(html.Toc) > 0 {
			_, err = h.book.AddSection(content, html.Toc[0].Text, html.ChapterID, h.contentCSS)
			if err != nil {
				return
			}
		} else {
			_, err = h.book.AddSection(content, "", html.ChapterID, h.contentCSS)
			if err != nil {
				return
			}
		}
	}
	return
}

func (h *HtmlToEpub) saveImages(doc *goquery.Document) (map[string]string, error) {
	downloads := make(map[string]string)

	tasks := request.NewDownloadTasks()
	doc.Find("img").Each(func(i int, img *goquery.Selection) {
		src, _ := img.Attr("src")
		if !strings.HasPrefix(src, "http") {
			return
		}

		_, exist := downloads[src]
		if exist {
			return
		}

		uri, err := url.Parse(src)
		if err != nil {
			log.Printf("parse %s fail: %s", src, err)
			return
		}
		_ = os.MkdirAll(h.ImagesDir, 0766)
		localFile := filepath.Join(h.ImagesDir, fmt.Sprintf("%s%s", MD5str(src), filepath.Ext(uri.Path)))

		tasks.Add(src, localFile)
		downloads[src] = localFile
	})
	// This API does not currently accept a caller context; widen it when HtmlToEpub gains one.
	if err := request.Batch(context.Background(), tasks, 3, time.Minute*2); err != nil {
		return downloads, fmt.Errorf("download EPUB assets: %w", err)
	}

	return downloads, nil
}

// TODO:
func (h *HtmlToEpub) getFontURLs(html HtmlContent) (downloads map[string]string, err error) {
	downloads = make(map[string]string)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html.Content))
	if err != nil {
		return
	}

	doc.Find("head>style").Each(func(i int, font *goquery.Selection) {
		fmt.Printf("%#v\n", font.Text())
		val, ok := font.Attr("font-family")
		fmt.Printf("%#v, %#v\n", val, ok)
		src, _ := font.Attr("url")
		if !strings.HasPrefix(src, "http") {
			return
		}

		_, exist := downloads[src]
		if exist {
			return
		}

		uri, err := url.Parse(src)
		if err != nil {
			log.Printf("parse %s fail: %s", src, err)
			return
		}
		_ = os.MkdirAll(h.FontsDir, 0766)
		localFile := filepath.Join(h.FontsDir, fmt.Sprintf("%s%s", MD5str(src), filepath.Ext(uri.Path)))

		downloads[src] = localFile
	})

	return
}

func (h *HtmlToEpub) changeRef(htmlFile string, img *goquery.Selection, refs, downloads map[string]string) {
	img.RemoveAttr("loading")
	img.RemoveAttr("srcset")

	src, _ := img.Attr("src")

	internalRef, exist := refs[src]
	if exist {
		img.SetAttr("src", internalRef)
		return
	}

	var localFile string
	switch {
	case strings.HasPrefix(src, "data:"):
		return
	case strings.HasPrefix(src, "http"):
		localFile, exist = downloads[src]
		if !exist {
			log.Printf("local file of %s not exist", src)
			return
		}
	default:
		fd, err := h.openLocalFile(htmlFile, src)
		if err != nil {
			log.Printf("local ref %s not found: %s", src, err)
			return
		}
		_ = fd.Close()
		localFile = fd.Name()
	}

	// check mime
	fmime, err := mimetype.DetectFile(localFile)
	{
		if err != nil {
			log.Printf("can't detect image mime of %s: %s", src, err)
			return
		}
		if !strings.HasPrefix(fmime.String(), "image") {
			log.Printf("mime of %s is %s instead of images", src, fmime.String())
			return
		}
	}

	// add image
	internalName := fmt.Sprintf("image_%03d", h.imgIdx)
	{
		h.imgIdx += 1
		if !strings.HasSuffix(internalName, fmime.Extension()) {
			internalName += fmime.Extension()
		}
		internalRef, err = h.book.AddImage(localFile, internalName)
		if err != nil {
			log.Printf("can't add image %s: %s", localFile, err)
			return
		}
		refs[src] = internalRef
	}

	if h.Verbose {
		log.Printf("replace %s as %s", src, localFile)
	}

	img.SetAttr("src", internalRef)
}

func (h *HtmlToEpub) openLocalFile(htmlFile string, ref string) (fd *os.File, err error) {
	fd, err = os.Open(ref)
	if err == nil {
		return
	}

	// compatible with evernote's exported htmls
	dirname := strings.TrimSuffix(htmlFile, filepath.Ext(htmlFile))
	name := filepath.Base(ref)
	fd, err = os.Open(filepath.Join(dirname+"_files", name))
	if err == nil {
		return
	}
	fd, err = os.Open(filepath.Join(dirname+".resources", name))
	if err == nil {
		return
	}
	if strings.HasSuffix(ref, ".") {
		return h.openLocalFile(htmlFile, strings.TrimSuffix(ref, "."))
	}

	return
}
