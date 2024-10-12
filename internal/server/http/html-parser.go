package http

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"golang.org/x/net/html"
)

// Helper function to update the href attribute of the base tag
func updateBaseHref(n *html.Node, newHref string) {
	if n.Type == html.ElementNode && n.Data == "base" {
		for i, attr := range n.Attr {
			if attr.Key == "href" {
				n.Attr[i].Val = newHref
				return
			}
		}
		// If the base tag doesn't have an href, add it
		n.Attr = append(n.Attr, html.Attribute{Key: "href", Val: newHref})
	}
}

// Traverse the HTML document to find and modify the base tag
func traverseAndModify(n *html.Node, newHref string) {
	if n == nil {
		return
	}
	updateBaseHref(n, newHref)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		traverseAndModify(c, newHref)
	}
}

// Convert the modified HTML back to a string
func renderHTML(n *html.Node) string {
	var b strings.Builder
	html.Render(&b, n)
	return b.String()
}

func (self *HttpServer) prepareIndexHtml() error {
	if self.baseHref == "" {
		return nil
	}

	if !strings.HasSuffix(self.baseHref, "/") || !strings.HasPrefix(self.baseHref, "/") {
		panic(fmt.Errorf("Base href must start and end with a /. For example: /my-base-href/"))
	}

	indexHtmlPath := "./public/index.html"
	indexHtml, err := os.Open(indexHtmlPath)
	if err != nil {
		return fmt.Errorf("Failed to open index.html for reading: %w", err)
	}

	htmlFile, err := html.Parse(indexHtml)
	if err != nil {
		return err
	}
	indexHtml.Close()

	traverseAndModify(htmlFile, self.baseHref)

	outputFile, err := os.Create(indexHtmlPath)
	if err != nil {
		return fmt.Errorf("Failed to open index.html for writting: %w", err)
	}
	defer outputFile.Close()
	_, err = outputFile.WriteString(renderHTML(htmlFile))

	if err != nil {
		return err
	}

	slog.Info("Base href set", slog.String("baseHref", self.baseHref))

	return nil
}
