package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	auth "github.com/abbot/go-http-auth"
	"github.com/dustin/go-humanize"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/html/charset"
)

type Email struct {
	No      int
	When    string
	Action  string
	Status  string
	Flow    string
	IP      string
	From    string
	To      string
	Queueid string
	Mid     string
	Score   string
	Size    string
	Subject string
	Sender  string
	Tests   string
}

func parseMaillog(c *gin.Context) {
	items := []Email{}
	no := 1
	var lines int64 = 0

	//
	CharsetReader := func(label string, input io.Reader) (io.Reader, error) {
		label = strings.Replace(label, "windows-", "cp", -1)
		encoding, _ := charset.Lookup(label)
		return encoding.NewDecoder().Reader(input), nil
	}
	dec := mime.WordDecoder{CharsetReader: CharsetReader}

	// files := os.Args[1:]
	files := flag.Args()
	if len(files) == 0 {
		files = []string{"/var/log/mail.log"}
	}

	for _, aFile := range files {
		f, err := os.Open(aFile)
		if err != nil {
			fmt.Println("There has been an error!: ", err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			lines++
			// filter request a
			line := scanner.Bytes()
			if bytes.Contains(line, []byte("amavis[")) {
				request := string(line)
				// fmt.Println(request)

				re := regexp.MustCompile(`(.*?\s+\d+ [\d:]+).*?\(([^\)]+)\) (Passed|Blocked) (.*?) {(.*?)}, .*?\[([^\s]+)\].*? [<]*([^\s>]*)[>] -> ([^\s]+), Queue-ID: ([^,]+)?, (Message-ID: [<]*([^\s>]*)[>],)?.*?Hits: ([^,]+), size: (\d+),.*?Subject: "(.*)", From: ([^,]+),.*?Tests: \[([^\s\]]*)\]?`)
				matches := re.FindStringSubmatch(request)
				// fmt.Printf("%q\n", matches)

				if len(matches) >= 16 {
					size, _ := strconv.ParseUint(matches[13], 10, 64)
					tests := strings.ReplaceAll(matches[16], ",", " ")

					subjectpart := strings.Split(matches[14], "(raw: ")
					subject := subjectpart[0]

					if len(subjectpart) > 1 {
						subjectraw := strings.TrimSuffix(subjectpart[1], ")")

						subject, _ = dec.DecodeHeader(subjectraw)
						if err != nil {
							subject = subjectpart[0]
						}
					}

					rcpt := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(matches[8], ",", " "), "<", " "), ">", " ")
					items = append([]Email{Email{no, matches[1], matches[3], matches[4], matches[5], matches[6], matches[7], rcpt, matches[9], matches[11], matches[12], humanize.Bytes(size), subject, matches[15], tests}}, items...)
					no++
				} else {
					fmt.Println(request)
					fmt.Printf("Not parsed: %v\n", request)
				}
			}

		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}

	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"title": "Amavis Watch",
		"lines": humanize.Comma(lines),
		"items": items,
	})
}

// BasicAuth gin middleware
func BasicAuth(a *auth.BasicAuth) gin.HandlerFunc {
	realmHeader := "Basic realm=" + strconv.Quote(a.Realm)

	return func(c *gin.Context) {
		user := a.CheckAuth(c.Request)

		if user == "" {
			c.Header("WWW-Authenticate", realmHeader)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Set("user", user)
	}
}

func main() {
	cred := flag.String("cred", "htpasswd.txt", "htpasswd credential file")
	release := flag.Bool("prod", true, "Run in production mode")
	flag.Parse()

	if *release {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.Static("/assets", "./assets")

	htpasswd := auth.HtpasswdFileProvider(*cred)
	authenticator := auth.NewBasicAuthenticator("Amavis Watch", htpasswd)
	authorized := r.Group("/", BasicAuth(authenticator))

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	authorized.GET("/index", parseMaillog)

	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
