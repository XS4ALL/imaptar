package main

import (
	"archive/tar"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/bgentry/speakeasy"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/klauspost/pgzip"
	"rsc.io/getopt"
)

func usage() {
	fmt.Fprint(os.Stderr,
		"\nUsage: imaptar <flags>\n\n"+
			"Flags:\n\n"+
			"    -s, --server <name>   IMAPS server name\n"+
			"    -u, --user <name>     username\n"+
			"    -t, --tar <file>      tar output filename\n\n"+
			"Optional flags:\n\n"+
			"    -p, --port <port>     IMAPS server port (default 993)\n"+
			"    -P, --pass <pass>     password\n"+
			"    -E, --envpass VAR     get password from environment var $VAR\n"+
			"    -z, --gzip            compress the output\n")
	os.Exit(1)
}
func main() {
	serverName := flag.String("server", "", "")
	serverPort := flag.String("port", "993", "")
	userName := flag.String("user", "", "")
	password := flag.String("pass", "", "")
	tarfile := flag.String("tar", "", "")
	envpass := flag.String("envpass", "", "")
	dogzip := flag.Bool("gzip", false, "")

	getopt.Aliases(
		"s", "server",
		"p", "port",
		"u", "user",
		"P", "pass",
		"t", "tar",
		"z", "gzip",
		"E", "envpass",
	)

	flag.Usage = usage
	getopt.Parse()

	if *serverName == "" || *userName == "" || *tarfile == "" {
		usage()
	}
	if *password == "" && *envpass != "" {
		p := os.Getenv(*envpass)
		if p == "" {
			log.Fatalf("environment var %s not set", *envpass)
		}
		password = &p
	}
	if *password == "" {
		p, err := speakeasy.Ask("Password: ")
		if err != nil {
			log.Fatal(err)
		}
		password = &p
	}

	var file io.WriteCloser
	var err error
	if *tarfile == "-" {
		file = os.Stdout
	} else {
		file, err = os.OpenFile(*tarfile, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			log.Fatal(err)
		}
	}
	if *dogzip {
		file = pgzip.NewWriter(file)
	}
	defer file.Close()

	tw := tar.NewWriter(file)
	defer tw.Close()

	// Connect to server
	log.Println("Connecting to server...")
	var c *client.Client
	if *serverPort == "143" {
		c, err = client.Dial(*serverName + ":" + *serverPort)
	} else {
		c, err = client.DialTLS(*serverName+":"+*serverPort, nil)
	}
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Connected to %s:%s\n", *serverName, *serverPort)

	// Don't forget to logout
	defer c.Logout()

	// Login
	if err := c.Login(*userName, *password); err != nil {
		log.Fatal(err)
	}
	log.Println("Logged in")

	// List mailboxes
	boxes := make(chan *imap.MailboxInfo, 10)
	go func() {
		if err := c.List("", "*", boxes); err != nil {
			log.Fatal(err)
		}
	}()

	var mailboxes []*imap.MailboxInfo
	for m := range boxes {
		mailboxes = append(mailboxes, m)
	}

	for _, m := range mailboxes {
		dumpFolder(*serverName, *userName, c, m.Name, tw)
	}
}
func mapFlags(imapflags []string) (mdflags string) {
	for _, f := range imapflags {
		switch f {
		case imap.SeenFlag:
			mdflags += "S"
		case imap.AnsweredFlag:
			mdflags += "R"
		case imap.FlaggedFlag:
			mdflags += "F"
		case imap.DeletedFlag:
			mdflags += "T"
		case imap.DraftFlag:
			mdflags += "D"
		case imap.RecentFlag:
		}
	}
	return
}
func dumpFolder(serverName string, userName string, c *client.Client, folderName string, tw *tar.Writer) {
	mbox, err := c.Select(folderName, false)
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("Selected %s, %d msgs", folderName, mbox.Messages)

	folderPath := ""
	if folderName != "INBOX" {
		folderPath = "." + strings.Replace(folderName, "/", ".", -1) + "/"
	}

	now := time.Now()
	hdr := tar.Header{
		Name:       folderPath,
		Mode:       0755,
		Size:       0,
		Typeflag:   tar.TypeDir,
		Uname:      userName,
		Gname:      userName,
		ModTime:    now,
		AccessTime: now,
		ChangeTime: now,
	}
	if folderPath != "" {
		err = tw.WriteHeader(&hdr)
		if err != nil {
			log.Fatal(err)
		}
	}
	hdr.Name = folderPath + "new/"
	err = tw.WriteHeader(&hdr)
	if err != nil {
		log.Fatal(err)
	}
	hdr.Name = folderPath + "tmp/"
	err = tw.WriteHeader(&hdr)
	if err != nil {
		log.Fatal(err)
	}
	hdr.Name = folderPath + "cur/"
	err = tw.WriteHeader(&hdr)
	if err != nil {
		log.Fatal(err)
	}

	if mbox.Messages == 0 {
		return
	}

	// Get all messages
	seqset := new(imap.SeqSet)
	seqset.AddRange(uint32(1), mbox.Messages)

	bodySectionName := imap.BodySectionName{
		BodyPartName: imap.BodyPartName{
			Specifier: imap.EntireSpecifier,
		},
		Peek: true,
	}
	msgs := make(chan *imap.Message, 10)
	go func() {
		err := c.Fetch(seqset, []imap.FetchItem{
			bodySectionName.FetchItem(),
			imap.FetchFlags,
			imap.FetchInternalDate,
			imap.FetchUid,
			imap.FetchRFC822Size}, msgs)
		if err != nil {
			log.Fatal(err)
		}
	}()

	for msg := range msgs {
		name := fmt.Sprintf("%d.%d_0.%s:2,%s",
			msg.InternalDate.Unix(), msg.Uid,
			serverName, mapFlags(msg.Flags))
		body := msg.GetBody(&bodySectionName)
		if body == nil {
			log.Printf("%s: uid %d: failed to retrieve body", folderName, msg.Uid)
			continue
		}
		log.Printf("write %s", name)
		hdr := tar.Header{
			Name:       folderPath + "cur/" + name,
			Mode:       0644,
			Size:       int64(body.Len()),
			Typeflag:   tar.TypeReg,
			Uname:      userName,
			Gname:      userName,
			ModTime:    msg.InternalDate,
			AccessTime: msg.InternalDate,
			ChangeTime: msg.InternalDate,
		}
		err := tw.WriteHeader(&hdr)
		if err != nil {
			log.Fatal(err)
		}

		_, err = io.Copy(tw, body)
		if err != nil {
			log.Fatal(err)
		}

		tw.Flush()
	}

	return
}
