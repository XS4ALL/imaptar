package main

import (
	"fmt"
	"log"
	"io"
	"os"
	"time"
	"strings"
	"flag"
	"archive/tar"

	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-imap"
)

func usage() {
	fmt.Fprintln(os.Stderr,
	"\nUsage: imaptar <flags>\n\n" +
	"Flags:\n\n" +
	"    -server <name>   IMAPS server name\n" +
	"    -port <port>     IMAPS server port (default 993)\n" +
	"    -user <name>     username\n" +
	"    -pass <pass>     password\n" +
	"    -tar <file>      tar output filename\n")
	os.Exit(1)
}

func main() {

	serverName := flag.String("server", "", "IMAPS server name")
	serverPort := flag.String("port", "993", "IMAPS server port")
	userName := flag.String("user", "", "username")
	password := flag.String("pass", "", "password")
	tarfile := flag.String("tar", "", "generated tar filename")
	flag.Parse()

	if *serverName == "" || *userName == "" || *password == "" || *tarfile == "" {
		usage()
	}

	file, err := os.OpenFile(*tarfile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal(err)
	}
	tw := tar.NewWriter(file)

	// Connect to server
	log.Println("Connecting to server...")
	var c *client.Client
	if *serverPort == "143" {
		c, err = client.Dial(*serverName + ":" + *serverPort)
	} else {
		c, err = client.DialTLS(*serverName + ":" + *serverPort, nil)
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
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func () {
		done <- c.List("", "*", mailboxes)
	}()

	for m := range mailboxes {
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
	log.Printf("Selected %s, %d msgs",
		folderName, mbox.Messages)

	if folderName == "INBOX" {
		folderName = ""
	} else {
		folderName = "." + strings.Replace(folderName, "/", ".", -1) + "/"
	}

	now := time.Now()
	hdr := tar.Header{
		Name:		folderName,
		Mode:		0755,
		Size:		0,
		Typeflag:	tar.TypeDir,
		Uname:		userName,
		Gname:		userName,
		ModTime:	now,
		AccessTime:	now,
		ChangeTime:	now,
	}
	if folderName != "" {
		err = tw.WriteHeader(&hdr)
		if err != nil {
			log.Fatal(err)
		}
	}
	hdr.Name = folderName + "new/"
	err = tw.WriteHeader(&hdr)
	if err != nil {
		log.Fatal(err)
	}
	hdr.Name = folderName + "tmp/"
	err = tw.WriteHeader(&hdr)
	if err != nil {
		log.Fatal(err)
	}
	hdr.Name = folderName + "cur/"
	err = tw.WriteHeader(&hdr)
	if err != nil {
		log.Fatal(err)
	}

	if mbox.Messages == 0 {
		return
	}

	// Get all messages
	from := uint32(1)
	to := mbox.Messages
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{
					imap.FetchFlags,
					imap.FetchRFC822,
					imap.FetchInternalDate,
					imap.FetchUid,
					imap.FetchRFC822Size}, messages)
	}()

	entireBody := imap.BodySectionName{
		BodyPartName:	imap.BodyPartName{
			Specifier:	imap.EntireSpecifier,
		},
		Peek:		true,
	}

	for msg := range messages {
		fn := fmt.Sprintf("%d.%d_0.%s:2,%s",
			msg.InternalDate.Unix(), msg.Uid,
			serverName, mapFlags(msg.Flags))
		lit := msg.GetBody(&entireBody)
		hdr := tar.Header{
			Name:		folderName + "cur/" + fn,
			Mode:		0644,
			Size:		int64(msg.Size),
			Typeflag:	tar.TypeReg,
			Uname:		userName,
			Gname:		userName,
			ModTime:	msg.InternalDate,
			AccessTime:	msg.InternalDate,
			ChangeTime:	msg.InternalDate,
		}
		err := tw.WriteHeader(&hdr)
		if err != nil {
			log.Fatal(err)
		}
		_, err = io.Copy(tw, lit)
		if err != nil {
			log.Fatal(err)
		}
		tw.Flush()
	}

	if err := <-done; err != nil {
		log.Fatal(err)
	}

	return
}

