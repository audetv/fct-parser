package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"golang.org/x/net/html"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type Topic struct {
	Question        Comment   `json:"question"`
	LinkedQuestions []Comment `json:"linked_question"`
	Comments        []Comment `json:"comments"`
}

type Comment struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	Text     string `json:"text"`
	Datetime string `json:"datetime"`
	DataID   string `json:"data_id,omitempty"`
}

func main() {

	var filename string
	boolPtr := flag.Bool("json", false, "a bool")
	flag.StringVar(&filename, "file", "topic.csv", "write to file name")
	flag.Parse()

	for _, url := range flag.Args() {
		doc, err := getTopicBody(url)
		if err != nil {
			log.Fatalf("parse: %v\n", err)
		}

		topic := Topic{}
		topic.parseTopic(doc)

		if *boolPtr {
			b, err := json.Marshal(topic)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%v\n\r", string(b))
		} else {
			writeCSVFile(topic, "./"+filename)
			fmt.Printf("The file ./%v was successeful writing\n", filename)
		}
	}
}

func getTopicBody(url string) (*html.Node, error) {
	resp, err := http.Get(url)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("getting %s: %s", url, resp.Status)
	}

	doc, err := html.Parse(resp.Body)
	resp.Body.Close()

	if err != nil {
		return nil, fmt.Errorf("parsing %s as HTML: %v", url, err)
	}

	return doc, nil
}

func checkError(message string, err error) {
	if err != nil {
		log.Fatal(message, err)
	}
}

func (topic *Topic) parseTopic(doc *html.Node) {
	parseQuestionView(doc, topic)
	parseCommentList(doc, topic)
}

func parseQuestionView(n *html.Node, topic *Topic) {

	exit := false

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && nodeHasRequiredCssClass("question-view", n) {
			topic.Question = parseComment(n)
		}
		if n.Type == html.ElementNode && nodeHasRequiredCssClass("linked-questions", n) {
			topic.LinkedQuestions = parseLinkedQuestions(n)
			exit = true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if exit == true {
				break
			}
			f(c)
		}
	}
	f(n)
}

func parseLinkedQuestions(n *html.Node) []Comment {
	var comments []Comment

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && nodeHasRequiredCssClass("linked-question", n) {
			comments = append(comments, parseComment(n))
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return comments
}

func parseCommentList(n *html.Node, topic *Topic) {
	var comments []Comment

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && nodeHasRequiredCssClass("comment-list", n) {
			// проходим по узлу с атрибутом class block comment-item}
			for cl := n.FirstChild; cl != nil; cl = cl.NextSibling {
				if cl.Type == html.ElementNode && nodeHasRequiredCssClass("comment-item", cl) {
					comments = append(comments, parseComment(cl))
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	topic.Comments = comments
}

func parseComment(n *html.Node) Comment {

	var nAnchor *html.Node
	var bufInnerHtml bytes.Buffer

	w := io.Writer(&bufInnerHtml)

	comment := Comment{}

	exit := false

	var f func(*html.Node)
	f = func(n *html.Node) {

		if n.Type == html.ElementNode && nodeHasRequiredCssClass("username", n) {
			comment.Username = getInnerText(n)
		}

		if n.Type == html.ElementNode && nodeHasRequiredCssClass("role", n) {
			comment.Role = getInnerText(n)
		}

		if n.Type == html.ElementNode && nodeHasRequiredCssClass("comment-text", n) {
			comment.DataID = getRequiredDataAttr("data-id", n)
			nAnchor = n
		}

		if n.Type == html.ElementNode && nodeHasRequiredCssClass("datetime", n) {
			comment.Datetime = strings.TrimSpace(getInnerText(n))
			exit = true
		}

		if nAnchor != nil {
			if n != nAnchor { // don't write the tag and its attributes
				html.Render(w, n)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if exit {
				break
			}
			f(c)
		}

		if n == nAnchor {
			comment.Text = bufInnerHtml.String()

			bufInnerHtml.Reset()
			nAnchor = nil
		}
	}
	f(n)

	return comment
}

func getInnerText(node *html.Node) string {
	for el := node.FirstChild; el != nil; el = el.NextSibling {
		if el.Type == html.TextNode {
			return el.Data
		}
	}
	return ""
}

func getRequiredDataAttr(rda string, n *html.Node) string {
	for _, attr := range n.Attr {
		if attr.Key == rda {
			return attr.Val
		}
	}
	return ""
}

// Перебирает аттрибуты токена в цикле и возвращает bool
// если в html token найден переданный css class
func nodeHasRequiredCssClass(rcc string, n *html.Node) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			classes := strings.Split(attr.Val, " ")
			for _, class := range classes {
				if class == rcc {
					return true
				}
			}
		}
	}
	return false
}

func writeCSVFile(topic Topic, outputPath string) {
	// Define header row
	headerRow := []string{
		"Username", "Role", "Text", "Datetime",
	}

	// Data array to write to CSV
	data := [][]string{
		headerRow,
	}

	data = append(data, []string{
		// Make sure the property order here matches
		// the one from 'headerRow' !!!
		topic.Question.Username,
		topic.Question.Role,
		topic.Question.Text,
		topic.Question.Datetime,
	})

	// Add linked question to output data
	for _, comment := range topic.LinkedQuestions {
		data = addCommentData(data, comment)
	}

	// Add comment list to output data
	for _, comment := range topic.Comments {
		data = addCommentData(data, comment)
	}

	// Create file
	file, err := os.Create(outputPath)
	checkError("Cannot create file", err)
	defer file.Close()

	// Create writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write rows into file
	for _, value := range data {
		err = writer.Write(value)
		checkError("Cannot write to file", err)
	}
}

func addCommentData(data [][]string, comment Comment) [][]string {
	return append(data, []string{
		// Make sure the property order here matches
		// the one from 'headerRow' !!!
		comment.Username,
		comment.Role,
		comment.Text,
		comment.Datetime,
	})
}
