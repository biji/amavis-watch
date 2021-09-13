package main

import (
	"bufio"
	"bytes"
	"flag"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	auth "github.com/abbot/go-http-auth"
	"github.com/dustin/go-humanize"
	"github.com/gin-gonic/gin"
	"github.com/kardianos/osext"
	"golang.org/x/net/html/charset"
)

type Email struct {
	No         int
	When       string
	Action     string
	Status     string
	Flow       string
	IP         string
	From       string
	To         string
	Queueid    string
	Mid        string
	Score      string
	Size       string
	Subject    string
	Sender     string
	SenderMail string
	Tests      string
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
			log.Print("There has been an error!: ", err)
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

				re := regexp.MustCompile(`(?P<when>.*?\s+\d+ [\d:]+).*?\(([^\)]+)\) (?P<action>Passed|Blocked) (?P<status>.*?) {(?P<flow>.*?)}.*?\[(?P<ip>[^\s]+)\]:\d+.*? [<]*(?P<from>[^\s>]*)[>] -> (?P<to>[^\s]+),( quarantine: [^\s]+,)? Queue-ID: (?P<queueid>[^,]+)?,( Message-ID: [<]*(?P<mid>[^\s>]*)[>],)? .*Hits: (?P<score>[^,]+), size: (?P<size>\d+),.*?Subject: "(?P<subject>.*)", From: (?P<sender>[^,]+)(?P<rest>.*)`)
				submatch := re.FindStringSubmatch(request)
				// fmt.Printf("%q\n", matches)

				if len(submatch) >= 16 {
					matches := make(map[string]string)
					for i, name := range re.SubexpNames() {
						if i != 0 && name != "" {
							matches[name] = submatch[i]
						}
					}

					size, _ := strconv.ParseUint(matches["size"], 10, 64)

					// tests
					re1 := regexp.MustCompile(`Tests: \[(?P<tests>[^\s\]]*)\]?`)
					match2 := re1.FindStringSubmatch(request)
					tests := ""
					if len(match2) > 1 {
						tests = strings.ReplaceAll(match2[1], ",", " ")
					}

					// status
					statuspart := strings.Split(matches["status"], " ")
					status := statuspart[0]

					// subject
					subjectpart := strings.Split(matches["subject"], "(raw: ")
					subject := subjectpart[0]

					if len(subjectpart) > 1 {
						subjectraw := strings.TrimSuffix(subjectpart[1], ")")
						subject, err = dec.DecodeHeader(subjectraw)
						if err != nil {
							log.Print("error decoding: " + subjectraw)
							subject = subjectpart[0]
						}
					}

					// sender
					r1 := regexp.MustCompile(` \(dkim:.*?\)$`)
					sender := r1.ReplaceAllString(matches["sender"], "")
					senderpart := strings.Split(sender, "(raw:_")
					sender = senderpart[0]
					if len(senderpart) > 1 {
						senderraw := strings.TrimSuffix(senderpart[1], ")")
						sender, err = dec.DecodeHeader(senderraw)
						if err != nil {
							log.Print("error decoding: " + senderraw)
							sender = senderpart[0]
						}
					}
					r2 := regexp.MustCompile(`"?(.*?)"?_<(.*?)>`)
					senderMatch := r2.FindStringSubmatch(sender)
					senderMail := ""
					if len(senderMatch) >= 3 {
						sender = senderMatch[1]
						senderMail = senderMatch[2]
					} else {
						senderMail = strings.Trim(sender, " _<>")
						sender = ""
					}
					sender = strings.ReplaceAll(sender, "_", " ")

					rcpt := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(matches["to"], ",", " "), "<", " "), ">", " ")
					items = append([]Email{Email{no, matches["when"], matches["action"], status, matches["flow"], matches["ip"], matches["from"], rcpt, matches["queueid"], matches["mid"], matches["score"], humanize.Bytes(size), subject, sender, senderMail, tests}}, items...)
					no++
				} else {
					// fmt.Println(request)
					log.Printf("Not parsed: %v", request)
				}
			}

		}
		if err := scanner.Err(); err != nil {
			log.Printf("Error: %v", err)
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

	folderPath, err := osext.ExecutableFolder()
	if err != nil {
		log.Fatal(err)
	}

	cred := flag.String("cred", "./htpasswd.txt", "htpasswd credential file")
	logOutput := flag.String("log", "", "Redirect log to this file")
	release := flag.Bool("prod", false, "Run in production mode")
	flag.Parse()

	if *release {
		gin.SetMode(gin.ReleaseMode)
	}

	if strings.Compare(*logOutput, "") != 0 {
		gin.DisableConsoleColor()
		f, err := os.OpenFile(*logOutput, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0664)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		gin.DefaultWriter = io.MultiWriter(f)
		log.SetOutput(f)
	}

	r := gin.Default()
	r.LoadHTMLGlob(folderPath + "/templates/*")
	r.Static("/assets", folderPath+"/assets")

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
